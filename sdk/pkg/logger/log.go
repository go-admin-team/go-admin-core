package logger

import (
	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// SetupLogger 简化的日志初始化（推荐使用 logger.NewFromConfig）
// Deprecated: 建议直接使用 logger.NewLogrusLogger 或 logger.NewFromConfig
func SetupLogger(opts ...logger.Option) logger.Logger {
	logger.DefaultLogger = logger.NewLogrusLogger(opts...)
	return logger.DefaultLogger
}
