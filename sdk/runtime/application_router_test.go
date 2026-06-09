package runtime

import (
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGetRouterConcurrentStable 实证 setRouter 的两项修复：
//  1. 并发安全：N 个 goroutine 并发调用 GetRouter()，go test -race 下不得报 data race
//     （修复前「写 e.routers」与「读返回值 e.routers」之间无 happens-before）。
//  2. 不重复累积：每次返回的路由数恒等于实际注册数，不随调用次数成倍膨胀
//     （修复前 setRouter 持续 append 同一个 e.routers，调用 K 次会得到 K 倍重复路由）。
func TestGetRouterConcurrentStable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ge := gin.New()
	noop := func(c *gin.Context) {}
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/reminder/team/members"},
		{http.MethodGet, "/api/v1/reminder/team/list"},
		{http.MethodGet, "/api/v1/reminder/team/total"},
		{http.MethodPost, "/api/v1/reminder/:id/urge"},
		{http.MethodGet, "/api/v1/sys-api"},
	}
	for _, r := range routes {
		ge.Handle(r.method, r.path, noop)
	}
	want := len(routes)

	app := NewConfig()
	app.SetEngine(ge)

	// 单次调用应恰好返回注册数（不多不少）。
	if got := len(app.GetRouter()); got != want {
		t.Fatalf("single GetRouter len = %d, want %d", got, want)
	}

	// 反复调用同一实例：仍应稳定等于注册数（验证不累积）。
	for i := 0; i < 10; i++ {
		if got := len(app.GetRouter()); got != want {
			t.Fatalf("repeated GetRouter #%d len = %d, want stable %d (accumulation bug)", i, got, want)
		}
	}

	// 并发调用：每次都应恰好返回注册数；配合 -race 验证无数据竞争。
	const goroutines, iters = 16, 50
	var wg sync.WaitGroup
	bad := make(chan int, goroutines*iters)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				if n := len(app.GetRouter()); n != want {
					bad <- n
				}
			}
		}()
	}
	wg.Wait()
	close(bad)
	if n, ok := <-bad; ok {
		t.Fatalf("concurrent GetRouter returned %d routers, want stable %d (accumulation/race bug)", n, want)
	}
}
