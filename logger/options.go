package logger

import (
	"context"
	"io"
)

type Option func(*Options)

type Options struct {
	// The logging level the logger should log at. default is `InfoLevel`
	Level Level
	// fields to always be logged
	Fields map[string]interface{}
	// It's common to set this to a file, or leave it default which is `os.Stderr`
	Out io.Writer
	// Caller skip frame count for file:line info
	CallerSkipCount int
	// Alternative options
	Context context.Context
	// Name logger name
	Name string
	// Timestamp.Format
	TimeFormat string
}

// WithFields set default fields for the logger
func WithFields(fields map[string]interface{}) Option {
	return func(args *Options) {
		args.Fields = fields
	}
}

func RemoveFieldByKey(key string) Option {
	return func(args *Options) {
		delete(args.Fields, key)
	}
}

// WithLevel set default level for the logger
func WithLevel(level Level) Option {
	return func(args *Options) {
		args.Level = level
	}
}

// WithOutput set default output writer for the logger
func WithOutput(out io.Writer) Option {
	return func(args *Options) {
		args.Out = out
	}
}

// WithCallerSkipCount set frame count to skip
func WithCallerSkipCount(c int) Option {
	return func(args *Options) {
		args.CallerSkipCount = c
	}
}

// WithName set name for logger
func WithName(name string) Option {
	return func(args *Options) {
		args.Name = name
	}
}

// WithTimeFormat set time format for logger eg. 2006-01-02 15:04:05.000Z0700
func WithTimeFormat(TimeFormat string) Option {
	return func(args *Options) {
		args.TimeFormat = TimeFormat
	}
}

func SetOption(k, v interface{}) Option {
	return func(o *Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, k, v)
	}
}
