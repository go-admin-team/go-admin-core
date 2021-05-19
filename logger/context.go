package logger

import "context"

type loggerKey struct{}

func FromContext(ctx context.Context) (*Helper, bool) {
	l, ok := ctx.Value(&loggerKey{}).(*Helper)
	return l, ok
}

func NewContext(ctx context.Context, l *Helper) context.Context {
	return context.WithValue(ctx, &loggerKey{}, l)
}
