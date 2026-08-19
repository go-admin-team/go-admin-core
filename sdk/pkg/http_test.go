package pkg

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Get 的四条性质，各由一个独家用例守住。
//
// 起因：原实现 `client := &http.Client{}`（Timeout 零值 = 无上界）+ 在 err 检查
// **之前**就 req.Header.Set。Get 的已知调用方是 go-admin-pro 的 HttpJob
// （cron HTTP 任务），跑在 cron 起的 goroutine 里：
//   - 对端挂死 → goroutine 永远回不来，按分钟调度就是每分钟泄漏一个
//   - url 来自用户填的 sys_job.invoke_target，填个畸形 URL → nil 解引用 panic

// TestGet_TimesOut 对端挂死时必须在有限时间内放弃。
//
// 独家杀死：退回 `&http.Client{}`（Timeout 零值），或把 getTimeout 改成 0。
// 下面三条用例的服务端都正常响应，对此完全无感。
func TestGet_TimesOut(t *testing.T) {
	// 刻意**不用** testing.Short() 跳过：那会让本次修复的核心性质在 CI 里永不验证。
	// 靠 get(url, timeout) 这个接缝把等待压到 200ms，用例既进 CI 又不拖慢它。
	const probe = 200 * time.Millisecond

	hang := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hang:
		case <-r.Context().Done():
		}
	}))
	defer func() { close(hang); srv.Close() }()

	done := make(chan error, 1)
	start := time.Now()
	go func() { _, err := get(srv.URL, probe); done <- err }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("端点根本没响应却成功了？")
		}
		if elapsed := time.Since(start); elapsed > probe*10 {
			t.Errorf("耗时 %v 远超设定的 %v", elapsed, probe)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("5s 仍未返回 —— 无超时的客户端又回来了")
	}
}

// TestGet_MalformedURLReturnsErrorNotPanic 畸形 URL 必须返错，不能 panic。
//
// 独家杀死：把 err 检查挪回 req.Header.Set 之后（原实现的形态）。
// 那时 NewRequest 失败返回 nil req，第一行 Header.Set 就空指针 panic ——
// 而这个 panic 发生在 cron 的 goroutine 里，url 又是用户在任务配置里填的。
//
// 用例本身就是探针：panic 会直接让它失败，不需要额外的 recover 断言。
func TestGet_MalformedURLReturnsErrorNotPanic(t *testing.T) {
	for _, bad := range []string{
		"://missing-scheme",
		"http://[::1]:namedport/x", // 非法端口
		"ht tp://has-space",
		string([]byte{0x7f}) + "://ctl-char",
	} {
		got, err := get(bad, time.Second)
		if err == nil {
			t.Errorf("URL %q 期望返错，实得 nil（返回 %q）", bad, got)
		}
	}
}

// TestGet_TruncatedBodyIsAnError body 读到一半断掉必须报错，不能当成功。
//
// 独家杀死：退回 `result, _ := io.ReadAll(...)`。那时调用方拿到的是**截断的内容**
// 且 err == nil —— 无从分辨自己拿到的是完整响应还是半截。
// 对 HttpJob 而言，这意味着一次实际失败的抓取被记成 success。
func TestGet_TruncatedBodyIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 声明 100 字节却只写 10 个，然后中止连接 → 客户端读 body 时拿到 unexpected EOF
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("0123456789"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler) // httptest 认得它，不会打印堆栈
	}))
	defer srv.Close()

	got, err := get(srv.URL, 5*time.Second)
	if err == nil {
		t.Errorf("body 被截断却返回成功（内容 %q）—— 读取错误被吞了", got)
	}
}

// TestGet_HappyPath 正常响应原样返回。
//
// 它守的是「上面三条不是靠把一切都判成失败来通过的」——
// 一个恒返错的 Get 能让前三条全绿，只有这条会红。
func TestGet_HappyPath(t *testing.T) {
	const body = `{"ok":true,"msg":"中文也要原样回来"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "*/*" {
			t.Errorf("Accept 头 = %q，期望 */*", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type 头 = %q，期望 application/json", got)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	got, err := get(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("正常请求失败: %v", err)
	}
	if got != body {
		t.Errorf("返回 %q，期望 %q", got, body)
	}
}

// TestGetTimeout_IsPositive 生产常量必须是正数。
//
// 上面四条用例全都**显式传了 timeout**（走 get 接缝），所以生产走的那条
// （Get → getTimeout）反而无人验证。这条把它补上 ——
// 否则「把 getTimeout 改成 0」会是一个所有测试都绿的缺陷。
func TestGetTimeout_IsPositive(t *testing.T) {
	if getTimeout <= 0 {
		t.Fatalf("getTimeout = %v —— 零或负数就是无上界，本次修复等于没做", getTimeout)
	}
}

// TestGet_UsesProductionTimeout Get 走的是真常量，不是某个小值。
//
// 独家杀死：`return get(url, 100*time.Millisecond)` 这类把生产超时改小的误接线。
// 服务端故意慢 300ms，传小值就会失败。
//
// 「传 0」那种误接线**不靠测试防，靠构造防**：get 内部 `timeout <= 0 → getTimeout`，
// 任何非正数都产生不了无界客户端。配合上一条（常量必为正），
// 「Get 拿到无上界客户端」这条路径已经不可达，不留待验证的缺口。
func TestGet_UsesProductionTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("slow-but-fine"))
	}))
	defer srv.Close()

	got, err := Get(srv.URL)
	if err != nil {
		t.Fatalf("Get 对 300ms 的正常响应失败了: %v —— 它传下去的超时太小", err)
	}
	if !strings.Contains(got, "slow-but-fine") {
		t.Errorf("返回 %q，期望含 slow-but-fine", got)
	}
}

// TestEffectiveTimeout_NeverUnbounded 非正数必须折成生产常量，绝不放行无上界。
//
// 独家杀死：删掉 effectiveTimeout 里的 `if d <= 0`。
//
// ⚠️ 这条用例是重写过的。原先写成「传 0 与传 getTimeout 对 300ms 响应都成功」，
// 试图从 get 的行为反推 —— 而**无上界客户端对 300ms 响应同样成功**，
// 那条断言恒真，删掉回落照样绿。变异验证抓到了它。
// 教训：当两个实现的行为差异只出现在「等很久之后」，就别用行为反推，
// 把判据折成一个纯函数直接看。
func TestEffectiveTimeout_NeverUnbounded(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Nanosecond, -time.Hour} {
		if got := effectiveTimeout(d); got != getTimeout {
			t.Errorf("effectiveTimeout(%v) = %v，期望回落到 getTimeout(%v)", d, got, getTimeout)
		}
		if effectiveTimeout(d) <= 0 {
			t.Errorf("effectiveTimeout(%v) 仍是非正数 —— 会造出无上界客户端", d)
		}
	}
	// 正数原样透传，证明回落不是「一律用常量」（那会让 get 的 timeout 参数失效，
	// 上面几条压缩到 200ms 的用例会集体变成真等 30s）
	for _, d := range []time.Duration{time.Nanosecond, time.Second, time.Hour} {
		if got := effectiveTimeout(d); got != d {
			t.Errorf("effectiveTimeout(%v) = %v，正数应原样透传", d, got)
		}
	}
}
