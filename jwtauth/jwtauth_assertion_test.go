package jwtauth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Unchecked type assertions used to turn malformed — but validly signed —
// tokens and misconfigured lookups into panics. They must surface as errors.

func newTestMiddleware(t *testing.T, key []byte, lookup string) *GinJWTMiddleware {
	t.Helper()
	mw := &GinJWTMiddleware{
		Realm:            "test",
		Key:              key,
		SigningAlgorithm: "HS256",
		Timeout:          time.Hour,
		MaxRefresh:       time.Hour,
		TokenLookup:      lookup,
		TokenHeadName:    "Bearer",
		TimeFunc:         time.Now,
	}
	if err := mw.MiddlewareInit(); err != nil {
		t.Fatalf("MiddlewareInit: %v", err)
	}
	return mw
}

func newTestContext(method, target string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, target, nil)
	return c
}

// A token signed with the right key but without orig_iat cannot be refreshed.
func TestCheckIfTokenExpireWithoutOrigIat(t *testing.T) {
	key := []byte("test-signing-key")
	mw := newTestMiddleware(t, key, "header: Authorization")

	tok := jwt.New(jwt.GetSigningMethod("HS256"))
	claims := tok.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	c := newTestContext(http.MethodGet, "/")
	c.Request.Header.Set("Authorization", "Bearer "+signed)

	_, err = mw.CheckIfTokenExpire(c)
	if !errors.Is(err, ErrMissingOrigIatField) {
		t.Errorf("want ErrMissingOrigIatField, got %v", err)
	}
}

// A TokenLookup entry without a colon must be skipped, not indexed blindly.
func TestParseTokenWithMalformedLookup(t *testing.T) {
	mw := newTestMiddleware(t, []byte("k"), "header")

	c := newTestContext(http.MethodGet, "/")
	if _, err := mw.ParseToken(c); err == nil {
		t.Error("want an error for a lookup with no usable entry")
	}
}

// A foreign value stored under JwtPayloadKey must not crash extraction.
func TestExtractClaimsWithForeignPayload(t *testing.T) {
	c := newTestContext(http.MethodGet, "/")
	c.Set(JwtPayloadKey, map[string]interface{}{"user": "admin"})

	if got := ExtractClaims(c); len(got) != 0 {
		t.Errorf("want empty claims for a foreign payload, got %v", got)
	}
}

func TestExtractClaimsFromTokenWithForeignClaims(t *testing.T) {
	tok := &jwt.Token{Claims: jwt.RegisteredClaims{}}

	if got := ExtractClaimsFromToken(tok); len(got) != 0 {
		t.Errorf("want empty claims for non-MapClaims token, got %v", got)
	}
}
