package user

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	jwt "github.com/go-admin-team/go-admin-core/v2/jwtauth"
)

func contextWith(claims jwt.MapClaims) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(jwt.JwtPayloadKey, claims)
	return c
}

// Every reader here used a bare type assertion, which panics on any type but
// the one it named — in a request handler, over claims whose shape a caller
// can influence. A claim that is not what was expected now reads as absent.
func TestClaimsOfTheWrongTypeDoNotPanic(t *testing.T) {
	c := contextWith(jwt.MapClaims{
		"identity": "not a number at all",
		"nice":     42.0,
		"rolekey":  []string{"unexpected"},
		"roleid":   map[string]int{"a": 1},
		"deptid":   true,
		"deptkey":  99.0,
	})

	if got := GetUserId(c); got != 0 {
		t.Errorf("GetUserId = %d, want 0", got)
	}
	if got := GetUserName(c); got != "" {
		t.Errorf("GetUserName = %q, want empty", got)
	}
	if got := GetRoleName(c); got != "" {
		t.Errorf("GetRoleName = %q, want empty", got)
	}
	if got := GetRoleId(c); got != 0 {
		t.Errorf("GetRoleId = %d, want 0", got)
	}
	if got := GetDeptId(c); got != 0 {
		t.Errorf("GetDeptId = %d, want 0", got)
	}
	if got := GetDeptName(c); got != "" {
		t.Errorf("GetDeptName = %q, want empty", got)
	}
}

// A float64 cannot hold an identity above 2^53 exactly, which is where
// snowflake ids live. json.Number keeps the digits, and a parser configured
// with UseNumber hands one over.
func TestAnIdentityBeyondFloat64PrecisionSurvives(t *testing.T) {
	const big = int64(1<<53) + 1

	if float64(big) == float64(big-1) {
		// Stated rather than assumed: this is the property that makes the
		// float64 path lossy in the first place.
		t.Logf("float64 cannot separate %d from %d", big, big-1)
	} else {
		t.Fatalf("%d is not beyond float64 precision; the test needs a larger value", big)
	}

	c := contextWith(jwt.MapClaims{"identity": json.Number("9007199254740993")})
	if got := GetUserIdStr(c); got != "9007199254740993" {
		t.Errorf("GetUserIdStr = %q, want the digits unchanged", got)
	}
}

func TestOrdinaryClaimsStillRead(t *testing.T) {
	c := contextWith(jwt.MapClaims{
		"identity": float64(7),
		"nice":     "alice",
		"rolekey":  "admin",
		"roleid":   float64(3),
		"deptid":   float64(5),
		"deptkey":  "hq",
	})

	if got := GetUserId(c); got != 7 {
		t.Errorf("GetUserId = %d, want 7", got)
	}
	if got := GetUserName(c); got != "alice" {
		t.Errorf("GetUserName = %q, want alice", got)
	}
	if got := GetRoleId(c); got != 3 {
		t.Errorf("GetRoleId = %d, want 3", got)
	}
	if got := GetDeptId(c); got != 5 {
		t.Errorf("GetDeptId = %d, want 5", got)
	}
	if got := GetDeptName(c); got != "hq" {
		t.Errorf("GetDeptName = %q, want hq", got)
	}
	_ = math.MaxInt64
}
