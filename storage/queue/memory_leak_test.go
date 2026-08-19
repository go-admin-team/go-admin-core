package queue

import (
	"runtime"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/storage"
)

// 这些测试锁定「队列写满时不得泄漏 goroutine」这一约定。
//
// 缺陷背景：Append 原本为每条消息启动一个 goroutine 执行 channel 写入。
// 队列满时该 goroutine 永久阻塞且无法退出，只要生产速度长期高于消费速度，
// goroutine 就会无上限累积直至 OOM。日志正是走的这条队列。
// 由 @Hemingway-ai 通过 pprof 定位（PR #89）。

func newTestMessage(stream string) storage.Messager {
	m := new(Message)
	m.SetStream(stream)
	m.SetValues(map[string]interface{}{"k": "v"})
	return m
}

// 向已满的队列持续投递，goroutine 数量不得随投递次数增长
func TestAppendDoesNotLeakGoroutines(t *testing.T) {
	// 容量为 1，投递第二条起队列即满
	m := NewMemory(1)

	if err := m.Append(newTestMessage("leak")); err != nil {
		t.Fatalf("首次投递不应失败: %v", err)
	}

	// 等待可能存在的异步投递完成，再取基线
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	before := runtime.NumGoroutine()

	// 队列已满，继续投递 500 条
	for i := 0; i < 500; i++ {
		_ = m.Append(newTestMessage("leak"))
	}

	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()

	// 留出少量余量以容忍运行时自身的 goroutine 波动
	if after-before > 10 {
		t.Errorf("投递 500 条消息后 goroutine 增加了 %d 个（%d → %d），存在泄漏",
			after-before, before, after)
	}
}

// 队列满时必须返回错误，而不是静默积压
func TestAppendReturnsErrorWhenFull(t *testing.T) {
	m := NewMemory(1)

	if err := m.Append(newTestMessage("full")); err != nil {
		t.Fatalf("首次投递不应失败: %v", err)
	}

	// 容量已用尽
	if err := m.Append(newTestMessage("full")); err == nil {
		t.Error("队列已满时 Append 返回了 nil，调用方无从得知消息被丢弃")
	}
}

// 未满时应正常投递并返回 nil
func TestAppendSucceedsWhenNotFull(t *testing.T) {
	m := NewMemory(8)

	for i := 0; i < 8; i++ {
		if err := m.Append(newTestMessage("ok")); err != nil {
			t.Fatalf("第 %d 次投递失败（容量 8，不应满）: %v", i+1, err)
		}
	}
}
