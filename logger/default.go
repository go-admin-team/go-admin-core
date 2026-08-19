package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	dlog "github.com/go-admin-team/go-admin-core/v2/observe/audit"
)

func init() {
	// 初始化默认日志级别
	// Initialize default log level
	lvl, err := GetLevel(os.Getenv("GO_ADMIN_LOG_LEVEL"))
	if err != nil {
		lvl = InfoLevel
	}

	// 创建默认日志记录器
	// Create default logger
	DefaultLogger = NewHelper(NewLogger(WithLevel(lvl)))
}

type defaultLogger struct {
	sync.RWMutex
	opts Options
}

// Init (opts...) should only overwrite provided options
// Init (opts...) 只应覆盖提供的选项
func (l *defaultLogger) Init(opts ...Option) error {
	for _, o := range opts {
		o(&l.opts)
	}
	return nil
}

// String 返回日志记录器的名称
// String returns the name of the logger
func (l *defaultLogger) String() string {
	return "default"
}

// Fields 设置默认字段
// Fields sets default fields
func (l *defaultLogger) Fields(fields map[string]interface{}) Logger {
	l.Lock()
	l.opts.Fields = copyFields(fields)
	l.Unlock()
	return l
}

// copyFields 复制字段
// copyFields copies fields
func copyFields(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// logCallerfilePath 返回调用者的包/文件:行信息
// returns a package/file:line description of the caller
func logCallerfilePath(loggingFilePath string) string {
	// To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.
	idx := strings.LastIndexByte(loggingFilePath, '/')
	if idx == -1 {
		return loggingFilePath
	}
	idx = strings.LastIndexByte(loggingFilePath[:idx], '/')
	if idx == -1 {
		return loggingFilePath
	}
	return loggingFilePath[idx+1:]
}

// Log 记录日志条目
// Log logs a log entry
func (l *defaultLogger) Log(level Level, v ...interface{}) {
	l.logf(level, "", v...)
}

// Logf 记录格式化的日志条目
// Logf logs a formatted log entry
func (l *defaultLogger) Logf(level Level, format string, v ...interface{}) {
	l.logf(level, format, v...)
}

// logf 记录日志条目
// logf logs a log entry
func (l *defaultLogger) logf(level Level, format string, v ...interface{}) {
	if !l.opts.Level.Enabled(level) {
		return
	}

	l.RLock()
	fields := copyFields(l.opts.Fields)
	l.RUnlock()

	if _, file, line, ok := runtime.Caller(l.opts.CallerSkipCount); ok && level == ErrorLevel {
		fields["file"] = fmt.Sprintf("%s:%d", logCallerfilePath(file), line)
	}

	rec := dlog.Record{
		Timestamp: time.Now(),
		Metadata:  make(map[string]string, len(fields)),
	}
	if format == "" {
		rec.Message = fmt.Sprint(v...)
	} else {
		rec.Message = fmt.Sprintf(format, v...)
	}

	for k, v := range fields {
		rec.Metadata[k] = fmt.Sprintf("%v", v)
	}

	var name string
	if l.opts.Name != "" {
		name = "[" + l.opts.Name + "]"
	}
	t := rec.Timestamp.Format("2006-01-02 15:04:05.000Z0700")
	logStr := fmt.Sprintf("%s %s %s %v\n", name, t, level.String(), rec.Message)
	if name == "" {
		logStr = fmt.Sprintf("%s %s %v\n", t, level.String(), rec.Message)
	}

	if _, err := l.opts.Out.Write([]byte(logStr)); err != nil {
		log.Printf("log [Logf] write error: %s \n", err.Error())
	}
}

// Options 返回日志记录器选项
// Options returns the logger options
func (l *defaultLogger) Options() Options {
	// not guard against options Context values
	l.RLock()
	opts := l.opts
	opts.Fields = copyFields(l.opts.Fields)
	l.RUnlock()
	return opts
}

// NewLogger 创建一个新的日志记录器
// NewLogger creates a new logger
func NewLogger(opts ...Option) Logger {
	// Default options
	options := Options{
		Level:           InfoLevel,
		Fields:          make(map[string]interface{}),
		Out:             os.Stderr,
		CallerSkipCount: 3,
		Context:         context.Background(),
		Name:            "",
	}

	l := &defaultLogger{opts: options}
	if err := l.Init(opts...); err != nil {
		l.Log(FatalLevel, err)
	}

	return l
}
