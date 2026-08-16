package jwtauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 这些测试锁定 token 续期的生命周期上限。
//
// 缺陷背景：RefreshToken 此前在生成新 token 时把 orig_iat 重置为当前时间，
// 而 CheckIfTokenExpire 正是依据 orig_iat 判断是否超出 MaxRefresh。两者相互
// 抵消，使 MaxRefresh 永远无法到达，token 可被无限续期
// （issue go-admin-team/go-admin#820）。
//
// 关于时间：ParseTokenString 调用 jwt.Parse 时未传入 jwt.WithTimeFunc，
// 因此 exp 始终按真实时钟校验，mw.TimeFunc 只影响 MaxRefresh 的判定。
// 测试据此以真实时间为基准，并将 Timeout 取得足够长，使 token 在真实时钟
// 下始终有效，从而把被测行为限定在 MaxRefresh 这一条路径上。

const (
	refreshTestKey = "test-secret-key-for-jwtauth-refresh"
	// 远长于测试模拟的时间跨度，确保 exp 在真实时钟下不会到期
	refreshTestTimeout = 72 * time.Hour
)

// newRefreshTestMiddleware 构造时钟可控的中间件
func newRefreshTestMiddleware(now func() time.Time, maxRefresh time.Duration) *GinJWTMiddleware {
	return &GinJWTMiddleware{
		Realm:            "test",
		Key:              []byte(refreshTestKey),
		SigningAlgorithm: "HS256",
		Timeout:          refreshTestTimeout,
		MaxRefresh:       maxRefresh,
		TimeFunc:         now,
		TokenLookup:      "header: Authorization",
		TokenHeadName:    "Bearer",
	}
}

// issueToken 签发一个带指定 orig_iat 的 token，模拟「签发于某个时刻」
func issueToken(t *testing.T, mw *GinJWTMiddleware, origIat time.Time) string {
	t.Helper()
	token := jwt.New(jwt.GetSigningMethod(mw.SigningAlgorithm))
	claims := token.Claims.(jwt.MapClaims)
	claims["identity"] = "tester"
	claims["exp"] = origIat.Add(refreshTestTimeout).Unix()
	claims["orig_iat"] = origIat.Unix()

	s, err := token.SignedString(mw.Key)
	if err != nil {
		t.Fatalf("签发测试 token 失败: %v", err)
	}
	return s
}

// requestWithToken 构造携带 token 的请求上下文
func requestWithToken(token string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req
	return c
}

// claimsOf 解析 token 取出 claims，跳过时间校验
func claimsOf(t *testing.T, mw *GinJWTMiddleware, tokenString string) jwt.MapClaims {
	t.Helper()
	parsed, err := jwt.Parse(
		tokenString,
		func(*jwt.Token) (interface{}, error) { return mw.Key, nil },
		jwt.WithoutClaimsValidation(),
	)
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	return parsed.Claims.(jwt.MapClaims)
}

// 续期后 orig_iat 必须保持不变 —— 它是 MaxRefresh 的计时起点
func TestRefreshTokenKeepsOrigIat(t *testing.T) {
	base := time.Now()
	now := base
	mw := newRefreshTestMiddleware(func() time.Time { return now }, 24*time.Hour)

	original := issueToken(t, mw, base)

	// 模拟 30 分钟后续期
	now = base.Add(30 * time.Minute)
	refreshed, _, err := mw.RefreshToken(requestWithToken(original))
	if err != nil {
		t.Fatalf("续期失败: %v", err)
	}

	got := int64(claimsOf(t, mw, refreshed)["orig_iat"].(float64))
	if got != base.Unix() {
		t.Errorf("orig_iat 被重置了：got %d, want %d（续期不应改变首次签发时间）", got, base.Unix())
	}
}

// 超出 MaxRefresh 后必须拒绝续期。
// 这是缺陷的核心：修复前无论经过多久，只要持续续期就永远不会被拒绝。
func TestRefreshTokenRejectedAfterMaxRefresh(t *testing.T) {
	base := time.Now()
	now := base
	const maxRefresh = 2 * time.Hour
	mw := newRefreshTestMiddleware(func() time.Time { return now }, maxRefresh)

	token := issueToken(t, mw, base)

	// 每 30 分钟续期一次，持续 4 小时 —— 远超 2 小时的上限
	var lastErr error
	for elapsed := 30 * time.Minute; elapsed <= 4*time.Hour; elapsed += 30 * time.Minute {
		now = base.Add(elapsed)

		newToken, _, err := mw.RefreshToken(requestWithToken(token))
		if err != nil {
			lastErr = err
			break
		}
		token = newToken
	}

	if lastErr == nil {
		t.Fatal("token 在超出 MaxRefresh 后仍可续期 —— 上限失效，可被无限续期")
	}
	if lastErr != ErrExpiredToken {
		t.Errorf("期望 ErrExpiredToken，实际为 %v", lastErr)
	}
}

// MaxRefresh 之内应当允许续期，确保修复没有把正常续期一并禁掉
func TestRefreshTokenAllowedWithinMaxRefresh(t *testing.T) {
	base := time.Now()
	now := base
	mw := newRefreshTestMiddleware(func() time.Time { return now }, 24*time.Hour)

	token := issueToken(t, mw, base)

	// 连续三次续期，均在上限之内，都应成功
	for i := 1; i <= 3; i++ {
		now = base.Add(time.Duration(i) * 30 * time.Minute)

		newToken, _, err := mw.RefreshToken(requestWithToken(token))
		if err != nil {
			t.Fatalf("第 %d 次续期被拒绝（仍在 MaxRefresh 之内）: %v", i, err)
		}
		token = newToken
	}
}

// 续期必须顺延 exp，否则新 token 一签发就是过期的
func TestRefreshTokenExtendsExpiry(t *testing.T) {
	base := time.Now()
	now := base
	mw := newRefreshTestMiddleware(func() time.Time { return now }, 24*time.Hour)

	token := issueToken(t, mw, base)

	now = base.Add(30 * time.Minute)
	refreshed, expire, err := mw.RefreshToken(requestWithToken(token))
	if err != nil {
		t.Fatalf("续期失败: %v", err)
	}

	wantExp := now.Add(refreshTestTimeout)
	if !expire.Equal(wantExp) {
		t.Errorf("返回的过期时间不正确：got %v, want %v", expire, wantExp)
	}

	got := int64(claimsOf(t, mw, refreshed)["exp"].(float64))
	if got != wantExp.Unix() {
		t.Errorf("token 中的 exp 不正确：got %d, want %d", got, wantExp.Unix())
	}
}
