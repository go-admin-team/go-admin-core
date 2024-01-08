package logger

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	loggerCore "github.com/go-admin-team/go-admin-core/logger"
)

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
		//l.Printf(l.infoStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
		log := l.getLogger(ctx)
		log.Logf(loggerCore.InfoLevel, l.infoStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// Warn print warn messages
func (l gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		//l.Printf(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
		log := l.getLogger(ctx)
		log.Logf(loggerCore.WarnLevel, l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

// Error print error messages
func (l gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		//l.Printf(l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
		log := l.getLogger(ctx)
		log.Logf(loggerCore.ErrorLevel, l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
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
				//l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
				log.Logf(loggerCore.TraceLevel, l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				//l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
				log.Logf(loggerCore.TraceLevel, l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}
		case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
			sql, rows := fc()
			slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
			if rows == -1 {
				//l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
				log.Logf(loggerCore.TraceLevel, l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				//l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
				log.Logf(loggerCore.TraceLevel, l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}
		case l.LogLevel == logger.Info:
			sql, rows := fc()
			if rows == -1 {
				//l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
				log.Logf(loggerCore.TraceLevel, l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				//l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
				log.Logf(loggerCore.TraceLevel, l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
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
		traceStr     = "%s\n[%.3fms] [rows:%v] %s\n"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s\n"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s\n"
	)

	if config.Colorful {
		infoStr = Green + "%s\n" + Reset + Green + "[info] " + Reset
		warnStr = BlueBold + "%s\n" + Reset + Magenta + "[warn] " + Reset
		errStr = Magenta + "%s\n" + Reset + Red + "[error] " + Reset
		traceStr = Green + "%s\n" + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s\n"
		traceWarnStr = Green + "%s " + Yellow + "%s\n" + Reset + RedBold + "[%.3fms] " + Yellow + "[rows:%v]" + Magenta + " %s" + Reset
		traceErrStr = RedBold + "%s " + MagentaBold + "%s\n" + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
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
