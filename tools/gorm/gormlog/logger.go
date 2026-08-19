package gormlog

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	loggerCore "github.com/go-admin-team/go-admin-core/v2/logger"
)

// fileWithLineNum 返回调用 GORM 的业务代码位置
// 复用 GORM 的 utils.FileWithLineNum() 跳过 gorm 内部代码
// 额外处理 gormlog 这一层的调用栈
func fileWithLineNum() string {
	// 先用 GORM 自己的方法获取（已跳过 gorm.io/gorm 内部）
	file := utils.FileWithLineNum()
	
	// 如果定位到 gormlog 包，继续向上查找真正的调用者
	if strings.Contains(file, "/gormlog/") {
		for i := 2; i < 15; i++ {
			_, f, line, ok := runtime.Caller(i)
			if ok && !strings.Contains(f, "/gormlog/") && !strings.Contains(f, "gorm.io/gorm") {
				return f + ":" + strconv.Itoa(line)
			}
		}
	}
	
	return file
}

// Colors
const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BlueBold    = "\033[34;1m"
	MagentaBold = "\033[35;1m"
	RedBold     = "\033[31;1m"
	YellowBold  = "\033[33;1m"
)

type gormLogger struct {
	logger.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

func (l *gormLogger) getLogger(ctx context.Context) loggerCore.Logger {
	requestId := ctx.Value("X-Request-Id")
	if requestId != nil {
		return loggerCore.DefaultLogger.Fields(map[string]interface{}{
			"x-request-id": requestId,
		})
	}
	return loggerCore.DefaultLogger
}

// LogMode log mode
func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info print info
func (l gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		log := l.getLogger(ctx)
		log.Log(loggerCore.InfoLevel, fmt.Sprintf(l.infoStr+msg, append([]interface{}{fileWithLineNum()}, data...)...))
	}
}

// Warn print warn messages
func (l gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		log := l.getLogger(ctx)
		log.Log(loggerCore.WarnLevel, fmt.Sprintf(l.warnStr+msg, append([]interface{}{fileWithLineNum()}, data...)...))
	}
}

// Error print error messages
func (l gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		log := l.getLogger(ctx)
		log.Log(loggerCore.ErrorLevel, fmt.Sprintf(l.errStr+msg, append([]interface{}{fileWithLineNum()}, data...)...))
	}
}

// Trace print sql message
func (l gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel > logger.Silent {
		log := l.getLogger(ctx)
		elapsed := time.Since(begin)
		switch {
		case err != nil && l.LogLevel >= logger.Error:
			sql, rows := fc()
			if rows == -1 {
				// 使用 Log 而不是 Logf，避免日志框架重复添加 caller
				log.Log(loggerCore.TraceLevel, fmt.Sprintf(l.traceErrStr, fileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql))
			} else {
				log.Log(loggerCore.TraceLevel, fmt.Sprintf(l.traceErrStr, fileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql))
			}
		case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
			sql, rows := fc()
			slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
			if rows == -1 {
				log.Log(loggerCore.TraceLevel, fmt.Sprintf(l.traceWarnStr, fileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql))
			} else {
				log.Log(loggerCore.TraceLevel, fmt.Sprintf(l.traceWarnStr, fileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql))
			}
		case l.LogLevel == logger.Info:
			sql, rows := fc()
			if rows == -1 {
				log.Log(loggerCore.TraceLevel, fmt.Sprintf(l.traceStr, fileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql))
			} else {
				log.Log(loggerCore.TraceLevel, fmt.Sprintf(l.traceStr, fileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql))
			}
		}
	}
}

type traceRecorder struct {
	logger.Interface
	BeginAt      time.Time
	SQL          string
	RowsAffected int64
	Err          error
}

func (l traceRecorder) New() *traceRecorder {
	return &traceRecorder{Interface: l.Interface, BeginAt: time.Now()}
}

func (l *traceRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.BeginAt = begin
	l.SQL, l.RowsAffected = fc()
	l.Err = err
}

func New(config logger.Config) logger.Interface {
	var (
		infoStr      = "%s\n[info] "
		warnStr      = "%s\n[warn] "
		errStr       = "%s\n[error] "
		traceStr     = "%s [%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s [%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s [%.3fms] [rows:%v] %s"
	)

	if config.Colorful {
		infoStr = Green + "%s " + Reset + Green + "[info] " + Reset
		warnStr = BlueBold + "%s " + Reset + Magenta + "[warn] " + Reset
		errStr = Magenta + "%s " + Reset + Red + "[error] " + Reset
		traceStr = Green + "%s " + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
		traceWarnStr = Green + "%s " + Yellow + "%s " + Reset + RedBold + "[%.3fms] " + Yellow + "[rows:%v]" + Magenta + " %s" + Reset
		traceErrStr = RedBold + "%s " + MagentaBold + "%s " + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
	}

	return &gormLogger{
		Config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}
}
