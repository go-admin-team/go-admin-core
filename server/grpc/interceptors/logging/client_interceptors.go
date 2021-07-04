/*
 * @Author: lwnmengjing
 * @Date: 2021/6/4 4:40 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/4 4:40 下午
 */

package logging

import (
	"context"
	"path"
	"time"

	"github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/server/grpc/interceptors/logging/ctxlog"
	"google.golang.org/grpc"
)

var (
	ClientField = ctxlog.NewFields("span.kind", "client")
)

// UnaryClientInterceptor returns a new unary client interceptor
// that optionally logs the execution of external gRPC calls.
func UnaryClientInterceptor(opts ...Option) grpc.UnaryClientInterceptor {
	o := evaluateClientOpt(opts)
	return func(
		ctx context.Context,
		method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption) error {

		fields := newClientLoggerFields(ctx, method)
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		logFinalClientLine(o, ctxlog.Extract(ctx).WithFields(fields.Values()), start, err,
			"finished client unary call")
		return err
	}
}

// StreamClientInterceptor returns a new streaming client interceptor
// that optionally logs the execution of external gRPC calls.
func StreamClientInterceptor(opts ...Option) grpc.StreamClientInterceptor {
	o := evaluateClientOpt(opts)
	return func(ctx context.Context, desc *grpc.StreamDesc,
		cc *grpc.ClientConn, method string,
		streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		fieles := newClientLoggerFields(ctx, method)
		start := time.Now()
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		logFinalClientLine(o, ctxlog.Extract(ctx).WithFields(fieles.Values()),
			start, err, "finished client streaming call")
		return clientStream, err
	}
}

func logFinalClientLine(o *options, l *logger.Helper, start time.Time, err error, msg string) {
	code := o.codeFunc(err)
	level := o.levelFunc(code)

	f := o.durationFunc(time.Now().Sub(start))
	f.Set("grpc.code", code)
	if err != nil {

	}
	l.WithFields(f.Values()).Log(level, msg, err)
}

func newClientLoggerFields(
	ctx context.Context,
	fullMethod string) ctxlog.Fields {
	service := path.Dir(fullMethod)[1:]
	method := path.Base(fullMethod)
	f := *SystemField
	f.Merge(ClientField)
	f.Set("grpc.service", service)
	f.Set("grpc.method", method)
	return f
}
