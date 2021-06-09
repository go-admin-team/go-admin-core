/*
 * @Author: lwnmengjing
 * @Date: 2021/6/4 10:27 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/4 10:27 上午
 */

package requesttag

import (
	"context"
	"github.com/go-admin-team/go-admin-core/tools/utils"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryServerInterceptor returns a new unary server interceptors that sets the values for request tags.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		return handler(AppendTagsForContext(ctx), req)
	}
}

// StreamServerInterceptor returns a new streaming server that sets the values for request tags.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {
		wrappedStream := grpc_middleware.WrapServerStream(stream)
		wrappedStream.WrappedContext = AppendTagsForContext(stream.Context())
		return handler(srv, wrappedStream)
	}
}

// AppendTagsForContext append RequestIDKey to context
func AppendTagsForContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx,
		utils.RequestIDKey, utils.GetRequestID(ctx),
	)
}

// UnaryClientInterceptor returns a new unary client interceptors that sets the values for request tags.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption) error {
		return invoker(AppendTagsForContext(ctx), method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor returns a new streaming client interceptors that sets the values for request tags.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string,
		streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(AppendTagsForContext(ctx), desc, cc, method, opts...)
	}
}
