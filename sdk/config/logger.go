package config

import (
	"fmt"
	"os"

	"github.com/go-admin-team/go-admin-core/v2/logger"
	log "github.com/go-admin-team/go-admin-core/v2/logger"
)

// RotationConfig 日志轮转配置
type RotationConfig struct {
	MaxSize    int  `yaml:"maxSize"`
	MaxAge     int  `yaml:"maxAge"`
	MaxBackups int  `yaml:"maxBackups"`
	Compress   bool `yaml:"compress"`
}

type Logger struct {
	Type         string          `yaml:"type"`
	Adapter      string          `yaml:"adapter"`
	Path         string          `yaml:"path"`
	Level        string          `yaml:"level"`
	Stdout       string          `yaml:"stdout"`
	Encoder      string          `yaml:"encoder"`
	EnableCaller bool            `yaml:"enableCaller"`
	EnabledDB    bool            `yaml:"enableddb"`
	Cap          uint            `yaml:"cap"`
	Rotation     *RotationConfig `yaml:"rotation"`
}

// adapterType 返回实际使用的适配器类型（优先使用 Adapter 字段）
func (e Logger) adapterType() string {
	if e.Adapter != "" {
		return e.Adapter
	}
	return e.Type
}

// Setup 设置logger（使用新的 logger 架构）
func (e Logger) Setup() {
	// 确保日志目录存在，如果无法创建则降级为控制台输出
	logPath := e.Path
	if logPath != "" {
		if err := os.MkdirAll(logPath, os.ModePerm); err != nil {
			fmt.Printf("[WARN] cannot create log dir %q: %v, falling back to console output\n", logPath, err)
			logPath = ""
		}
	}

	// 构建选项
	opts := []logger.Option{
		logger.WithName("go-admin"),
		logger.WithCallerSkipCount(2),
	}

	// 设置日志级别
	if level, err := logger.GetLevel(e.Level); err == nil {
		opts = append(opts, logger.WithLevel(level))
	}

	// 设置输出模式
	if e.Stdout == "file" || e.Stdout == "" {
		opts = append(opts, logger.WithStdout(false))
	} else {
		opts = append(opts, logger.WithStdout(true))
	}

	// 设置文件路径（仅当目录创建成功时）
	if logPath != "" {
		opts = append(opts, logger.WithPath(logPath))
	}

	// 设置日志格式
	if e.Encoder != "" {
		opts = append(opts, logger.WithEncoder(e.Encoder))
	}

	// 设置调用者信息
	opts = append(opts, logger.WithEnableCaller(e.EnableCaller))

	// 设置日志轮转配置（仅在有文件输出时生效）
	if logPath != "" && e.Rotation != nil {
		opts = append(opts, logger.WithRotation(
			e.Rotation.MaxSize,
			e.Rotation.MaxAge,
			e.Rotation.MaxBackups,
			e.Rotation.Compress,
		))
	}

	// 选择适配器
	switch e.adapterType() {
	case "zap":
		log.DefaultLogger = logger.NewZapLogger(opts...)
	default:
		log.DefaultLogger = logger.NewLogrusLogger(opts...)
	}
}

var LoggerConfig = new(Logger)
