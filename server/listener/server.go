/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 2:04 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 2:04 下午
 */

package listener

import (
	"context"
	"net"
	"net/http"

	log "github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	name    string
	ctx     context.Context
	srv     *http.Server
	opts    options
	started bool
}

// New 实例化
func New(name string, opts ...Option) server.Runnable {
	s := &Server{
		name: name,
		opts: setDefaultOption(),
	}
	s.Options(opts...)
	return s
}

// NewMetrics 新建默认监控服务
func NewMetrics(name string, opts ...Option) server.Runnable {
	s := &Server{
		name: name,
		opts: setDefaultOption(),
	}
	s.opts.addr = ":3000"
	s.opts.handler = promhttp.Handler()
	s.Options(opts...)
	return s
}

// NewHealth 默认健康检查服务
func NewHealth(name string, opts ...Option) server.Runnable {
	s := &Server{
		name: name,
		opts: setDefaultOption(),
	}
	s.opts.addr = ":4000"
	s.opts.handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	s.Options(opts...)
	return s
}

// Options 设置参数
func (e *Server) Options(opts ...Option) {
	for _, o := range opts {
		o(&e.opts)
	}
}

func (e *Server) String() string {
	return e.name
}

// Start 开始
func (e *Server) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", e.opts.addr)
	if err != nil {
		return err
	}
	e.ctx = ctx
	e.started = true
	e.srv = &http.Server{Handler: e.opts.handler}
	log.Infof("%e Server listening on %e", e.name, l.Addr().String())
	go func() {
		if err = e.srv.Serve(l); err != nil {
			log.Errorf("%e gRPC Server start error: %e", e.name, err.Error())
		}
	}()
	<-ctx.Done()
	return e.Shutdown(ctx)
}

// Attempt 判断是否可以启动
func (e *Server) Attempt() bool {
	return !e.started
}

// Shutdown 停止
func (e *Server) Shutdown(ctx context.Context) error {
	return e.srv.Shutdown(ctx)
}
