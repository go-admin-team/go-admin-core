package jwtauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// 这些测试锁定 token 续期的生命周期上限。
//
// 缺陷背景：RefreshToken 此前在生成新 token 时把 orig_iat 重置为当前时间，
// 而 CheckIfTokenExpire 正是依据 orig_iat 判断是否超出 MaxRefresh。两者相互
// 抵消，使 MaxRefresh 永远无法到达，token 可被无限续期
// （issue go-admin-team/go-admin#820）。

const testKey = "test-secret-key-for-jwtauth"

// newTestMiddleware 构造一个时间可控的中间件，避免测试依赖真实时钟
func newTestMiddleware(now func() time.Time, timeout, maxRefresh time.Duration) *GinJWTMiddleware {
	return &GinJWTMiddleware{
		Realm:            "test",
		Key:              []byte(testKey),
		SigningAlgorithm: "HS256",
		Timeout:          timeout,
		MaxRefresh:       maxRefresh,
		TimeFunc:         now,
		TokenLookup:      "header: Authorization",
		TokenHeadName:    "Bearer",
	}
}

// makeToken 直接签发一个带指定 orig_iat 的 token，模拟「签发于某个时刻」
func makeToken(t *testing.T, mw *GinJWTMiddleware, origIat, exp time.Time) string {
	t.Helper()
	token := jwt.New(jwt.GetSigningMethod(mw.SigningAlgorithm))
	claims := token.Claims.(jwt.MapClaims)
	claims["identity"] = "tester"
	claims["exp"] = exp.Unix()
	claims["orig_iat"] = origIat.Unix()

	s, err := token.SignedString(mw.Key)
	if err != nil {
		t.Fatalf("签发测试 token 失败: %v", err)
	}
	return s
}

// ctxWithToken 构造一个携带 token 的请求上下文
func ctxWithToken(token string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req
	return c
}

func origIatOf(t *testing.T, mw *GinJWTMiddleware, tokenString string) int64 {
	t.Helper()
	parsed, err := jwt.Parse(tokenString, func(*jwt.Token) (interface{}, error) {
		return mw.Key, nil
	})
	// token 可能已过期，过期不影响读取 claims
	if parsed == nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	claims := MapClaims(parsed.Claims.(jwt.MapClaims))
	v, err := claims.OrigIat()
	if err != nil {
		t.Fatalf("读取 orig_iat 失败: %v", err)
	}
	return v
}

// 续期后 orig_iat 必须保持不变 —— 它是 MaxRefresh 的计时起点
func TestRefreshTokenKeepsOrigIat(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	mw := newTestMiddleware(func() time.Time { return now }, time.Hour, 24*time.Hour)

	original := makeToken(t, mw, base, base.Add(time.Hour))

	// 30 分钟后续期
	now = base.Add(30 * time.Minute)
	refreshed, _, err := mw.RefreshToken(ctxWithToken(original))
	if err != nil {
		t.Fatalf("续期失败: %v", err)
	}

	if got, want := origIatOf(t, mw, refreshed), base.Unix(); got != want {
		t.Errorf("orig_iat 被重置了：got %d, want %d（续期不应改变首次签发时间）", got, want)
	}
}

// 超出 MaxRefresh 后必须拒绝续期。
// 这是缺陷的核心：修复前无论经过多久，只要持续续期就永远不会被拒绝。
func TestRefreshTokenRejectedAfterMaxRefresh(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	const maxRefresh = 2 * time.Hour
	mw := newTestMiddleware(func() time.Time { return now }, time.Hour, maxRefresh)

	token := makeToken(t, mw, base, base.Add(time.Hour))

	// 每 30 分钟续期一次，持续 4 小时 —— 远超 2 小时的 MaxRefresh
	var lastErr error
	for elapsed := 30 * time.Minute; elapsed <= 4*time.Hour; elapsed += 30 * time.Minute {
		now = base.Add(elapsed)

		newToken, _, err := mw.RefreshToken(ctxWithToken(token))
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
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	mw := newTestMiddleware(func() time.Time { return now }, time.Hour, 24*time.Hour)

	token := makeToken(t, mw, base, base.Add(time.Hour))

	// 即使原 token 已过期（超过 Timeout），只要在 MaxRefresh 内仍可续期
	now = base.Add(90 * time.Minute)
	if _, _, err := mw.RefreshToken(ctxWithToken(token)); err != nil {
		t.Fatalf("MaxRefresh 之内的续期被拒绝: %v", err)
	}
}

// 续期必须刷新 exp，否则新 token 一签发就是过期的
func TestRefreshTokenExtendsExpiry(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	const timeout = time.Hour
	mw := newTestMiddleware(func() time.Time { return now }, timeout, 24*time.Hour)

	token := makeToken(t, mw, base, base.Add(timeout))

	now = base.Add(30 * time.Minute)
	refreshed, expire, err := mw.RefreshToken(ctxWithToken(token))
	if err != nil {
		t.Fatalf("续期失败: %v", err)
	}

	if want := now.Add(timeout); !expire.Equal(want) {
		t.Errorf("过期时间未按当前时刻顺延：got %v, want %v", expire, want)
	}

	parsed, _ := jwt.Parse(refreshed, func(*jwt.Token) (interface{}, error) {
		return mw.Key, nil
	})
	claims := parsed.Claims.(jwt.MapClaims)
	if got := int64(claims["exp"].(float64)); got != want(now, timeout) {
		t.Errorf("token 中的 exp 不正确：got %d, want %d", got, want(now, timeout))
	}
}

func want(now time.Time, timeout time.Duration) int64 {
	return now.Add(timeout).Unix()
}
