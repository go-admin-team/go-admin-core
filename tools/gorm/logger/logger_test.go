package logger

import (
	"context"
	"testing"
	"time"

	logCore "github.com/go-admin-team/go-admin-core/logger"
	"gorm.io/gorm/logger"
)

func TestNew(t *testing.T) {
	l := New(logger.Config{
		SlowThreshold: time.Second,
		Colorful:      true,
		LogLevel: logger.LogLevel(
			logCore.DefaultLogger.Options().Level.LevelForGorm()),
	})
	l.Info(context.TODO(), "test")
}
