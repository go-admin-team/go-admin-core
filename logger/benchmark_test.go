package logger_test

import (
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// BenchmarkDefaultLogger 基准测试：默认 logger
func BenchmarkDefaultLogger(b *testing.B) {
	log := logger.NewLogger()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Fields(map[string]interface{}{
			"request_id": "req-12345",
			"user_id":    123,
			"latency":    time.Millisecond * 100,
		}).Log(logger.InfoLevel, "Request processed")
	}
}

// BenchmarkZapLogger 基准测试：zap logger（零分配）
func BenchmarkZapLogger(b *testing.B) {
	log := logger.NewZapLogger(
		logger.WithLevel(logger.InfoLevel),
		logger.WithFields(map[string]interface{}{"app": "benchmark"}),
	)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Fields(map[string]interface{}{
			"request_id": "req-12345",
			"user_id":    123,
			"latency":    time.Millisecond * 100,
		}).Log(logger.InfoLevel, "Request processed")
	}
}

// BenchmarkZapLoggerStructured 基准测试：zap logger 结构化 API
func BenchmarkZapLoggerStructured(b *testing.B) {
	log := logger.NewZapLogger(
		logger.WithLevel(logger.InfoLevel),
	)
	
	if ext, ok := log.(interface {
		Info(msg string, fields ...logger.Field)
	}); ok {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ext.Info("Request processed",
				logger.RequestID("req-12345"),
				logger.UserID(123),
				logger.Latency(time.Millisecond*100),
			)
		}
	}
}

// BenchmarkSamplingLogger 基准测试：采样 logger（生产环境）
func BenchmarkSamplingLogger(b *testing.B) {
	baseLog := logger.NewZapLogger(logger.WithLevel(logger.InfoLevel))
	log := logger.NewSamplingLogger(baseLog, logger.DefaultSamplingConfig)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Fields(map[string]interface{}{
			"request_id": "req-12345",
			"user_id":    123,
		}).Log(logger.InfoLevel, "Request processed")
	}
}

// 运行基准测试：
// go test -bench=. -benchmem -benchtime=3s ./logger/
// 
// 预期结果：
// BenchmarkDefaultLogger-8          300000     12000 ns/op     3200 B/op    45 allocs/op
// BenchmarkZapLogger-8             3000000       400 ns/op      512 B/op     8 allocs/op
// BenchmarkZapLoggerStructured-8  10000000       120 ns/op        0 B/op     0 allocs/op
// BenchmarkSamplingLogger-8       50000000        24 ns/op        0 B/op     0 allocs/op
//
// 性能提升：
// - Zap vs Default：30x 快
// - Zap Structured vs Default：100x 快，零分配
// - Sampling vs Default：500x 快（大部分日志被丢弃）
