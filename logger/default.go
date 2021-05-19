package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	dlog "github.com/yahao333/go-admin-core/debug/log"
)

func init() {
	lvl, err := GetLevel(os.Getenv("GO_ADMIN_LOG_LEVEL"))
	if err != nil {
		lvl = InfoLevel
	}

	DefaultLogger = NewHelper(NewLogger(WithLevel(lvl)))
}

type defaultLogger struct {
	sync.RWMutex
	opts Options
}

// Init(opts...) should only overwrite provided options
func (l *defaultLogger) Init(opts ...Option) error {
	for _, o := range opts {
		o(&l.opts)
	}
	return nil
}

func (l *defaultLogger) String() string {
	return "default"
}

func (l *defaultLogger) Fields(fields map[string]interface{}) Logger {
	l.Lock()
	l.opts.Fields = copyFields(fields)
	l.Unlock()
	return l
}

func copyFields(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// logCallerfilePath returns a package/file:line description of the caller,
// preserving only the leaf directory name and file name.
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

func (l *defaultLogger) Log(level Level, v ...interface{}) {
	l.logf(level, "", v...)
}

func (l *defaultLogger) Logf(level Level, format string, v ...interface{}) {
	l.logf(level, format, v...)
}

func (l *defaultLogger) logf(level Level, format string, v ...interface{}) {
	// TODO decide does we need to write message if log level not used?
	if !l.opts.Level.Enabled(level) {
		return
	}

	l.RLock()
	fields := copyFields(l.opts.Fields)
	l.RUnlock()

	fields["level"] = level.String()

	if _, file, line, ok := runtime.Caller(l.opts.CallerSkipCount); ok {
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

	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		keys = append(keys, k)
		rec.Metadata[k] = fmt.Sprintf("%v", v)
	}

	sort.Strings(keys)
	metadata := ""

	for _, k := range keys {
		metadata += fmt.Sprintf(" %s: %v", k, fields[k])
	}

	var name string
	if l.opts.Name != "" {
		name = "[" + l.opts.Name + "]"
	}
	t := rec.Timestamp.Format("2006-01-02T15:04:05.000Z0700")
	//fmt.Printf("%s %s %v\n", t, metadata, rec.Message)
	_, err := l.opts.Out.Write([]byte(fmt.Sprintf("%s %s %s %v\n", name, t, metadata, rec.Message)))
	if err != nil {
		log.Printf("log [Logf] write error: %s \n", err.Error())
	}
}

func (l *defaultLogger) Options() Options {
	// not guard against options Context values
	l.RLock()
	opts := l.opts
	opts.Fields = copyFields(l.opts.Fields)
	l.RUnlock()
	return opts
}

// NewLogger builds a new logger based on options
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
