/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 5:29 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 5:29 下午
 */

package grpc

import (
	"context"
	"fmt"
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	log "github.com/ht-qukuai-yjingy/go-admin-core/logger"
	"github.com/ht-qukuai-yjingy/go-admin-core/server/grpc/interceptors/logging"
	reqtags "github.com/ht-qukuai-yjingy/go-admin-core/server/grpc/interceptors/request_tag"
	"google.golang.org/grpc"
)

type Service struct {
	Connection  *grpc.ClientConn
	CallTimeout time.Duration
}

func (e *Service) Dial(
	endpoint string,
	callTimeout time.Duration,
	unary ...grpc.UnaryClientInterceptor) (err error) {
	log.Infof("configure service with endpoint: %s", endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()

	if len(unary) == 0 {
		unary = defaultUnaryClientInterceptors()
	}
	e.Connection, err = grpc.DialContext(ctx,
		endpoint,
		grpc.WithInsecure(),
		grpc.WithStreamInterceptor(middleware.ChainStreamClient(defaultStreamClientInterceptors()...)),
		grpc.WithUnaryInterceptor(middleware.ChainUnaryClient(unary...)),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true), grpc.MaxCallRecvMsgSize(defaultMaxMsgSize)),
	)

	if err != nil {
		msg := fmt.Sprintf("connect gRPC service %s failed", endpoint)
		log.Errorf(msg, err)
		return fmt.Errorf("%w, "+msg, err)
	}
	return nil
}

func defaultUnaryClientInterceptors() []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{
		opentracing.UnaryClientInterceptor(),
		logging.UnaryClientInterceptor(),
		reqtags.UnaryClientInterceptor(),
	}
}

func defaultStreamClientInterceptors() []grpc.StreamClientInterceptor {
	return []grpc.StreamClientInterceptor{
		opentracing.StreamClientInterceptor(),
		logging.StreamClientInterceptor(),
		reqtags.StreamClientInterceptor(),
	}
}
