package config

import "github.com/go-admin-team/go-admin-core/sdk/pkg/logger"

type Logger struct {
	Type       string
	Path       string
	Level      string
	Stdout     string
	EnabledDB  bool
	Cap        uint
	DaysToKeep uint
}

// Setup 设置logger
func (e Logger) Setup() {
	logger.SetupLogger(
		logger.WithType(e.Type),
		logger.WithPath(e.Path),
		logger.WithLevel(e.Level),
		logger.WithStdout(e.Stdout),
		logger.WithCap(e.Cap),
		logger.WithDaysToKeep(e.DaysToKeep),
	)
}

var LoggerConfig = new(Logger)
