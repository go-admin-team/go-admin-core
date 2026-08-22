package user

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"

	jwtauth "github.com/go-admin-team/go-admin-core/v2/jwtauth"
)

// identityBeyondFloat64 is 2^53 + 1: the first integer float64 cannot represent
// exactly. Snowflake ids live well above it, and any system that does not use
// auto-increment primary keys hands out identities of this size.
const identityBeyondFloat64 = "9007199254740993"

// The accessors read json.Number where it is present, but nothing produces one
// unless the parser is told to. Without the option the claim is a float64 and
// the digits are gone before any accessor is reached, so this has to be tested
// through the parser rather than by handing a json.Number to claimInt.
func TestLargeIdentitySurvivesTheParser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := []byte("a-key-for-this-test-only")

	signed, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, gojwt.MapClaims{
		"identity": int64(9007199254740993),
		"nice":     "someone",
	}).SignedString(key)
	if err != nil {
		t.Fatalf("sign the token: %v", err)
	}

	mw := &jwtauth.GinJWTMiddleware{
		SigningAlgorithm: "HS256",
		Key:              key,
		TokenLookup:      "header: Authorization",
		TokenHeadName:    "Bearer",
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+signed)

	claims, err := mw.GetClaimsFromJWT(c)
	if err != nil {
		t.Fatalf("parse the token: %v", err)
	}
	c.Set(jwtauth.JwtPayloadKey, claims)

	if got := GetUserIdStr(c); got != identityBeyondFloat64 {
		t.Fatalf("identity came back as %s, want %s", got, identityBeyondFloat64)
	}
	if got := GetUserId(c); int64(got) != 9007199254740993 {
		t.Fatalf("identity came back as %d, want 9007199254740993", got)
	}
}
