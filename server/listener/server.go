/*
 * @Author: lwnmengjing
 * @Date: 2021/6/8 2:04 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/8 2:04 下午
 */

package listener

import (
	"context"
	"github.com/go-admin-team/go-admin-core/server"
	"net"
	"net/http"

	log "github.com/go-admin-team/go-admin-core/logger"
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
	for i := range opts {
		opts[i](&s.opts)
	}
	return s
}

func (s *Server) String() string {
	return s.name
}

// Start 开始
func (s *Server) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", s.opts.addr)
	if err != nil {
		return err
	}
	s.ctx = ctx
	s.started = true
	s.srv = &http.Server{Handler: s.opts.handler}
	log.Infof("%s Server listening on %s", s.name, l.Addr().String())
	go func() {
		if err = s.srv.Serve(l); err != nil {
			log.Errorf("%s gRPC Server start error: %s", s.name, err.Error())
		}
	}()
	<-ctx.Done()
	return s.srv.Serve(l)
}

// Attempt 判断是否可以启动
func (s *Server) Attempt() bool {
	return !s.started
}

// Shutdown 停止
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
