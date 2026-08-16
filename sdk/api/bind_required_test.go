package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// An empty request body must not bypass binding:"required" (issue #81).

type loginReq struct {
	User     string `json:"user" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func newBindTestApi(body string) *Api {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	return (&Api{}).MakeContext(c)
}

func TestBindRejectsEmptyBodyWithRequiredFields(t *testing.T) {
	e := newBindTestApi("")

	req := loginReq{}
	err := e.Bind(&req).Errors

	if err == nil {
		t.Error("empty body passed validation: required was bypassed and a " +
			"zero-valued struct would reach the business logic")
	}
}

func TestBindRejectsPartialBody(t *testing.T) {
	e := newBindTestApi(`{"user":"admin"}`)

	req := loginReq{}
	if err := e.Bind(&req).Errors; err == nil {
		t.Error("a body missing password passed validation")
	}
}

// Guards against the fix rejecting well-formed requests.
func TestBindAcceptsCompleteBody(t *testing.T) {
	e := newBindTestApi(`{"user":"admin","password":"secret"}`)

	req := loginReq{}
	if err := e.Bind(&req).Errors; err != nil {
		t.Fatalf("complete request was rejected: %v", err)
	}
	if req.User != "admin" || req.Password != "secret" {
		t.Errorf("fields were not bound: %+v", req)
	}
}
