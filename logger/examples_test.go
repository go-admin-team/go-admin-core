package logger_test

import (
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// 示例 1：使用默认 Logrus Logger
func ExampleNewLogrusLogger() {
	log := logger.NewLogrusLogger(
		logger.WithLevel(logger.InfoLevel),
	)

	// 使用结构化日志 API
	if ext, ok := log.(interface {
		Info(msg string, fields ...logger.Field)
		Warn(msg string, fields ...logger.Field)
	}); ok {
		ext.Info("Application started")
		ext.Warn("This is a warning")
		ext.Info("User logged in",
			logger.Int("user_id", 123),
			logger.FieldString("action", "login"),
		)
	}
}

// 示例 2：从配置创建 Logger
func ExampleNewFromConfig() {
	cfg := &logger.Config{
		Core: logger.CoreConfig{
			Level: "info",
			Name:  "my-service",
		},
		Adapter: logger.AdapterConfig{
			Type: "logrus",
		},
		Output: logger.OutputConfig{
			Console: &logger.ConsoleConfig{
				Enabled: true,
				Color:   true,
			},
			File: &logger.FileConfig{
				Enabled:    true,
				Path:       "logs/app.log",
				MaxSize:    100,
				MaxBackups: 7,
				Compress:   true,
			},
		},
	}

	log, err := logger.NewFromConfig(cfg)
	if err != nil {
		panic(err)
	}

	// 使用结构化日志 API
	if ext, ok := log.(interface{ Info(msg string, fields ...logger.Field) }); ok {
		ext.Info("Logger created from config")
	}
}

// 示例 3：开发环境 Logger
func ExampleNewDevelopmentLogger() {
	log := logger.NewDevelopmentLogger()

	// 使用结构化日志 API
	if ext, ok := log.(interface {
		Debug(msg string, fields ...logger.Field)
		Info(msg string, fields ...logger.Field)
		Error(msg string, fields ...logger.Field)
	}); ok {
		ext.Debug("Debug information")
		ext.Info("Application running")
		ext.Error("Something went wrong")
	}
}

// 示例 4：生产环境 Logger（带采样）
func ExampleNewProductionLogger() {
	log := logger.NewProductionLogger("logs/production.log")

	// 使用结构化日志 API
	if ext, ok := log.(interface{ Info(msg string, fields ...logger.Field) }); ok {
		// 高频日志会被采样
		for i := 0; i < 1000; i++ {
			ext.Info("Processing request")
		}
	}

	// Error 级别不会被采样
	if ext, ok := log.(interface{ Error(msg string, fields ...logger.Field) }); ok {
		ext.Error("Critical error occurred")
	}
}

// 示例 5：结构化字段
func ExampleNewLogrusLogger_structuredFields() {
	log := logger.NewLogrusLogger()

	// 使用结构化字段 API（推荐）
	if ext, ok := log.(interface {
		Info(msg string, fields ...logger.Field)
	}); ok {
		ext.Info("HTTP Request",
			logger.RequestID("req-12345"),
			logger.UserID(123),
			logger.Method("GET"),
			logger.URI("/api/users"),
			logger.StatusCode(200),
			logger.Latency(100*time.Millisecond),
			logger.ClientIP("192.168.1.1"),
		)
	}
}

// 示例 6：添加 Logrus Hook（生态优势）
func ExampleNewLogrusLogger_addHook() {
	log := logger.NewLogrusLogger()

	// 类型断言获取 Logrus 特定功能
	if logrusAdapter, ok := log.(interface {
		AddHook(hook interface{})
	}); ok {
		// 添加 Sentry Hook（错误上报）
		// sentryHook := logger.NewSentryHook()
		// logrusAdapter.AddHook(sentryHook)

		// 添加 Elasticsearch Hook（日志存储）
		// esHook := logger.NewElasticsearchHook(esClient, "logs")
		// logrusAdapter.AddHook(esHook)

		// 添加 Metrics Hook（监控）
		metricsHook := logger.NewMetricsHook()
		logrusAdapter.AddHook(metricsHook)
	}

	if ext, ok := log.(interface{ Error(msg string, fields ...logger.Field) }); ok {
		ext.Error("This error will trigger hooks")
	}
}

// 示例 7：从旧配置迁移（向后兼容）
func ExampleFromLegacyConfig() {
	legacyCfg := &logger.LegacyConfig{
		Path:      "temp/logs",
		Stdout:    "",
		Level:     "info",
		EnabledDB: false,
	}

	cfg := logger.FromLegacyConfig(legacyCfg)
	log, err := logger.NewFromConfig(cfg)
	if err != nil {
		panic(err)
	}

	if ext, ok := log.(interface{ Info(msg string, fields ...logger.Field) }); ok {
		ext.Info("Migrated from legacy config")
	}
}

// 示例 8：注册自定义适配器
func ExampleRegisterAdapter() {
	// 注册自定义适配器
	logger.RegisterAdapter("custom", func(opts ...logger.Option) logger.Logger {
		// 实现自定义 Logger
		return logger.NewLogger(opts...)
	})

	// 使用自定义适配器
	cfg := &logger.Config{
		Adapter: logger.AdapterConfig{
			Type: "custom",
		},
	}

	log, _ := logger.NewFromConfig(cfg)
	if ext, ok := log.(interface{ Info(msg string, fields ...logger.Field) }); ok {
		ext.Info("Using custom adapter")
	}
}

// 基准测试：Logrus vs Default
func BenchmarkLogrusLogger(b *testing.B) {
	log := logger.NewLogrusLogger(
		logger.WithLevel(logger.InfoLevel),
	)

	ext, ok := log.(interface{ Info(msg string, fields ...logger.Field) })
	if !ok {
		b.Fatal("Logger does not support Info method")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext.Info("Request processed",
			logger.FieldString("request_id", "req-12345"),
			logger.Int("user_id", 123),
		)
	}
}

// 运行测试：
// go test -bench=. -benchmem ./logger/
