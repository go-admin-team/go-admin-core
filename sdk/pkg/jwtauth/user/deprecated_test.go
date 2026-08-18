package user_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/jwtauth"
	newuser "github.com/go-admin-team/go-admin-core/jwtauth/user"
	olduser "github.com/go-admin-team/go-admin-core/sdk/pkg/jwtauth/user"
)

// withClaims builds a context carrying the payload the accessors read.
func withClaims(claims jwtauth.MapClaims) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(jwtauth.JwtPayloadKey, claims)
	return c
}

// Every accessor has to return what the package it forwards to returns. A shim
// that compiles but reads a different key would be worse than no shim at all,
// because the importer would see plausible zero values instead of an error.
func TestForwardsToTheCurrentPackage(t *testing.T) {
	claims := jwtauth.MapClaims{
		"identity": float64(7),
		"nice":     "seven",
		"nickname": "tester",
		"rolekey":  "admin",
		"roleid":   float64(3),
		"deptid":   float64(11),
		"deptkey":  "ops",
	}

	cases := []struct {
		name string
		old  func(*gin.Context) interface{}
		want func(*gin.Context) interface{}
	}{
		{"GetUserId",
			func(c *gin.Context) interface{} { return olduser.GetUserId(c) },
			func(c *gin.Context) interface{} { return newuser.GetUserId(c) }},
		{"GetUserIdStr",
			func(c *gin.Context) interface{} { return olduser.GetUserIdStr(c) },
			func(c *gin.Context) interface{} { return newuser.GetUserIdStr(c) }},
		{"GetUserName",
			func(c *gin.Context) interface{} { return olduser.GetUserName(c) },
			func(c *gin.Context) interface{} { return newuser.GetUserName(c) }},
		{"GetRoleName",
			func(c *gin.Context) interface{} { return olduser.GetRoleName(c) },
			func(c *gin.Context) interface{} { return newuser.GetRoleName(c) }},
		{"GetRoleId",
			func(c *gin.Context) interface{} { return olduser.GetRoleId(c) },
			func(c *gin.Context) interface{} { return newuser.GetRoleId(c) }},
		{"GetDeptId",
			func(c *gin.Context) interface{} { return olduser.GetDeptId(c) },
			func(c *gin.Context) interface{} { return newuser.GetDeptId(c) }},
		{"GetDeptName",
			func(c *gin.Context) interface{} { return olduser.GetDeptName(c) },
			func(c *gin.Context) interface{} { return newuser.GetDeptName(c) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, want := tc.old(withClaims(claims)), tc.want(withClaims(claims))
			if got != want {
				t.Errorf("got %#v, want %#v", got, want)
			}
			// A zero value on both sides would make the comparison meaningless.
			if got == nil || got == "" || got == 0 {
				t.Errorf("the fixture does not exercise this accessor: got %#v", got)
			}
		})
	}

	if got := olduser.ExtractClaims(withClaims(claims)); len(got) != len(claims) {
		t.Errorf("ExtractClaims: got %d claims, want %d", len(got), len(claims))
	}
	if got := olduser.Get(withClaims(claims), "nickname"); got != "tester" {
		t.Errorf("Get: got %#v, want tester", got)
	}
}
