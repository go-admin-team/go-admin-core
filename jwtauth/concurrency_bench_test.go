package jwtauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Every authenticated request pays for a token parse before any business code
// runs, so this is the floor under the API's throughput. The claim set matches
// what go-admin actually issues (identity, roleid, rolekey, nice, datascope,
// rolename, deptid) - claim count drives both the signature size and the map
// allocation, so a smaller set would report a number nobody sees in practice.

const benchKey = "bench-secret-key-for-jwtauth-throughput"

// newBenchMiddlewareForTest mirrors newBenchMiddleware for callers holding a
// *testing.T rather than a *testing.B.
func newBenchMiddlewareForTest(t *testing.T) *GinJWTMiddleware {
	t.Helper()
	mw := benchMiddleware()
	if err := mw.MiddlewareInit(); err != nil {
		t.Fatal(err)
	}
	return mw
}

// benchMiddleware builds the middleware without initialising it, so the two
// wrappers above can report failure through their own testing type.
func benchMiddleware() *GinJWTMiddleware {
	return &GinJWTMiddleware{
		Realm:            "bench",
		Key:              []byte(benchKey),
		SigningAlgorithm: "HS256",
		Timeout:          time.Hour,
		MaxRefresh:       time.Hour,
		TokenLookup:      "header: Authorization",
		TokenHeadName:    "Bearer",
		PayloadFunc: func(data interface{}) MapClaims {
			return MapClaims{
				IdentityKey:  1,
				RoleIdKey:    1,
				RoleKey:      "admin",
				NiceKey:      "admin",
				DataScopeKey: "2",
				RoleNameKey:  "System Administrator",
				"deptid":     1,
			}
		},
	}
}

func newBenchMiddleware(b *testing.B) *GinJWTMiddleware {
	b.Helper()
	mw := benchMiddleware()
	if err := mw.MiddlewareInit(); err != nil {
		b.Fatal(err)
	}
	return mw
}

func benchToken(b *testing.B, mw *GinJWTMiddleware) string {
	b.Helper()
	token, _, err := mw.TokenGenerator(nil)
	if err != nil {
		b.Fatal(err)
	}
	return token
}

// BenchmarkTokenGenerator is the login path: signing happens once per login,
// not once per request.
func BenchmarkTokenGenerator(b *testing.B) {
	mw := newBenchMiddleware(b)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, _, err := mw.TokenGenerator(nil); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkParseTokenString is the per-request cost: HMAC verification plus
// claim decoding, with no HTTP or routing around it.
func BenchmarkParseTokenString(b *testing.B) {
	mw := newBenchMiddleware(b)
	token := benchToken(b, mw)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := mw.ParseTokenString(token); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkMiddlewareFunc measures what a protected route costs before it
// reaches a handler: gin routing, header extraction, parse, claim injection.
// The handler does nothing, so the difference against an unprotected route is
// the price of authentication.
func BenchmarkMiddlewareFunc(b *testing.B) {
	gin.SetMode(gin.TestMode)
	mw := newBenchMiddleware(b)
	token := benchToken(b, mw)

	r := gin.New()
	r.GET("/ping", mw.MiddlewareFunc(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		for pb.Next() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Errorf("status %d", w.Code)
				return
			}
		}
	})
}

// BenchmarkBareRoute is the control: the same route without the middleware, so
// the authentication cost can be read as a difference rather than an absolute.
func BenchmarkBareRoute(b *testing.B) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		for pb.Next() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Errorf("status %d", w.Code)
				return
			}
		}
	})
}
