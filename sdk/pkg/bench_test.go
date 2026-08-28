package pkg

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Two helpers run on essentially every request. GenerateMsgIDFromContext gives
// the request its correlation id, and GetOrm is how handlers and services reach
// the database - go-admin calls it from fourteen places, several of them more
// than once per request.

func benchContext(withHeader bool) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if withHeader {
		c.Request.Header.Set(TrafficKey, "11111111-2222-3333-4444-555555555555")
	}
	return c
}

// BenchmarkGenerateMsgIDFromContextNew is a request that arrived without a
// correlation id: a UUID is minted and written to the response header.
//
// One context serves every iteration because the lookup reads the *request*
// header while the write goes to the *response* header, so the miss repeats
// and every iteration mints a fresh id.
func BenchmarkGenerateMsgIDFromContextNew(b *testing.B) {
	c := benchContext(false)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateMsgIDFromContext(c)
	}
}

// BenchmarkGenerateMsgIDFromContextExisting is every later call, and the case
// where a gateway already supplied the id: a header read and nothing else.
func BenchmarkGenerateMsgIDFromContextExisting(b *testing.B) {
	c := benchContext(true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateMsgIDFromContext(c)
	}
}

func BenchmarkGetOrm(b *testing.B) {
	c := benchContext(true)
	c.Set("db", new(gorm.DB))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := GetOrm(c); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetOrmMissing is the error path. It matters because the handlers
// call GetOrm before doing anything else, so a misconfigured request pays this
// on every route.
func BenchmarkGetOrmMissing(b *testing.B) {
	c := benchContext(true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := GetOrm(c); err == nil {
			b.Fatal("expected an error")
		}
	}
}
