/*
 * @Author: lwnmengjing
 * @Date: 2021/6/2 4:30 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/2 4:30 下午
 */

package grpc

import (
	"context"
	"crypto/tls"
	"math"
	"time"

	pbErr "github.com/go-admin-team/go-admin-core/errors"
	log "github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/server/grpc/interceptors/logging"
	requesttag "github.com/go-admin-team/go-admin-core/server/grpc/interceptors/request_tag"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

const (
	infinity                           = time.Duration(math.MaxInt64)
	defaultMaxMsgSize                  = 4 << 20
	defaultMaxConcurrentStreams        = 100000
	defaultKeepAliveTime               = 30 * time.Second
	defaultConnectionIdleTime          = 10 * time.Second
	defaultMaxServerConnectionAgeGrace = 10 * time.Second
	defaultMiniKeepAliveTimeRate       = 2
)

type Option func(*Options)

type Options struct {
	id                       string
	domain                   string
	addr                     string
	tls                      *tls.Config
	keepAlive                time.Duration
	timeout                  time.Duration
	maxConnectionAge         time.Duration
	maxConnectionAgeGrace    time.Duration
	maxConcurrentStreams     int
	maxMsgSize               int
	unaryServerInterceptors  []grpc.UnaryServerInterceptor
	streamServerInterceptors []grpc.StreamServerInterceptor
	ctx                      context.Context
}

func WithContextOption(c context.Context) Option {
	return func(o *Options) {
		o.ctx = c
	}
}

func WithIDOption(s string) Option {
	return func(o *Options) {
		o.id = s
	}
}

func WithDomainOption(s string) Option {
	return func(o *Options) {
		o.domain = s
	}
}

func WithAddrOption(s string) Option {
	return func(o *Options) {
		o.addr = s
	}
}

func WithTlsOption(tls *tls.Config) Option {
	return func(o *Options) {
		o.tls = tls
	}
}

func WithKeepAliveOption(t time.Duration) Option {
	return func(o *Options) {
		o.keepAlive = t
	}
}

func WithTimeoutOption(t time.Duration) Option {
	return func(o *Options) {
		o.keepAlive = t
	}
}

func WithMaxConnectionAgeOption(t time.Duration) Option {
	return func(o *Options) {
		o.maxConnectionAge = t
	}
}

func WithMaxConnectionAgeGraceOption(t time.Duration) Option {
	return func(o *Options) {
		o.maxConnectionAgeGrace = t
	}
}

func WithMaxConcurrentStreamsOption(i int) Option {
	return func(o *Options) {
		o.maxConcurrentStreams = i
	}
}

func WithMaxMsgSizeOption(i int) Option {
	return func(o *Options) {
		o.maxMsgSize = i
	}
}

func WithUnaryServerInterceptorsOption(u ...grpc.UnaryServerInterceptor) Option {
	return func(o *Options) {
		if o.unaryServerInterceptors == nil {
			o.unaryServerInterceptors = make([]grpc.UnaryServerInterceptor, 0)
		}
		o.unaryServerInterceptors = append(o.unaryServerInterceptors, u...)
	}
}

func WithStreamServerInterceptorsOption(u ...grpc.StreamServerInterceptor) Option {
	return func(o *Options) {
		if o.streamServerInterceptors == nil {
			o.streamServerInterceptors = make([]grpc.StreamServerInterceptor, 0)
		}
		o.streamServerInterceptors = append(o.streamServerInterceptors, u...)
	}
}

func defaultOptions() *Options {
	return &Options{
		addr:                  ":0",
		keepAlive:             defaultKeepAliveTime,
		timeout:               defaultConnectionIdleTime,
		maxConnectionAge:      infinity,
		maxConnectionAgeGrace: defaultMaxServerConnectionAgeGrace,
		maxConcurrentStreams:  defaultMaxConcurrentStreams,
		maxMsgSize:            defaultMaxMsgSize,
		unaryServerInterceptors: []grpc.UnaryServerInterceptor{
			requesttag.UnaryServerInterceptor(),
			ctxtags.UnaryServerInterceptor(),
			opentracing.UnaryServerInterceptor(),
			logging.UnaryServerInterceptor(),
			prometheus.UnaryServerInterceptor,
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(customRecovery("", ""))),
		},
		streamServerInterceptors: []grpc.StreamServerInterceptor{
			requesttag.StreamServerInterceptor(),
			ctxtags.StreamServerInterceptor(),
			opentracing.StreamServerInterceptor(),
			logging.StreamServerInterceptor(),
			prometheus.StreamServerInterceptor,
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(customRecovery("", ""))),
		},
	}
}

func customRecovery(id, domain string) recovery.RecoveryHandlerFunc {
	return func(p interface{}) (err error) {
		log.Errorf("panic triggered: %v", p)
		return pbErr.New(id, domain, pbErr.InternalServerError)
	}
}
