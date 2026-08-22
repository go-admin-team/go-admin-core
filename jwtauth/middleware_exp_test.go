package jwtauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Nothing covered the guarded path itself, so a change to how exp is read
// could reject every authenticated request without a test noticing. The
// middleware asserted float64 there; with the parser decoding numbers as
// json.Number that assertion fails and every request is answered as a
// malformed exp.
func TestMiddlewareLetsAValidTokenThrough(t *testing.T) {
	mw := newRefreshTestMiddleware(time.Now, time.Hour)
	if err := mw.MiddlewareInit(); err != nil {
		t.Fatalf("init the middleware: %v", err)
	}
	token := issueToken(t, mw, time.Now())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	reached := false
	r.GET("/guarded", mw.MiddlewareFunc(), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !reached {
		t.Fatalf("a valid token was turned away: status %d, handler reached %v, body %s",
			w.Code, reached, w.Body.String())
	}
}
