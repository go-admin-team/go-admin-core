/*
 * @Author: lwnmengjing
 * @Date: 2021/6/7 5:43 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/7 5:43 下午
 */

package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-admin-team/go-admin-core/logger"
)

type Server struct {
	services               map[string]Runnable
	mutex                  sync.Mutex
	errChan                chan error
	waitForRunnable        sync.WaitGroup
	internalCtx            context.Context
	internalCancel         context.CancelFunc
	internalProceduresStop chan struct{}
	shutdownCtx            context.Context
	shutdownCancel         context.CancelFunc
	logger                 *logger.Helper
	opts                   options
}

// New 实例化
func New(opts ...Option) Manager {
	s := &Server{
		services:               make(map[string]Runnable),
		errChan:                make(chan error),
		internalProceduresStop: make(chan struct{}),
	}
	s.opts = setDefaultOptions()
	for i := range opts {
		opts[i](&s.opts)
	}
	return s
}

// Add add runnable
func (e *Server) Add(r ...Runnable) {
	if e.services == nil {
		e.services = make(map[string]Runnable)
	}
	for i := range r {
		e.services[r[i].String()] = r[i]
	}
}

// Start start runnable
func (e *Server) Start(ctx context.Context) (err error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.internalCtx, e.internalCancel = context.WithCancel(ctx)
	stopComplete := make(chan struct{})
	defer close(stopComplete)
	defer func() {
		stopErr := e.engageStopProcedure(stopComplete)
		if stopErr != nil {
			if err != nil {
				err = fmt.Errorf("%s, %w", stopErr.Error(), err)
			} else {
				err = stopErr
			}
		}
	}()
	e.errChan = make(chan error)

	for k := range e.services {
		if !e.services[k].Attempt() {
			//先判断是否可以启动
			return errors.New("can't accept new runnable as stop procedure is already engaged")
		}
	}
	//按顺序启动
	for k := range e.services {
		e.startRunnable(e.services[k])
	}
	e.waitForRunnable.Wait()
	select {
	case <-ctx.Done():
		return nil
	case err := <-e.errChan:
		return err
	}
}

func (e *Server) startRunnable(r Runnable) {
	e.waitForRunnable.Add(1)
	go func() {
		defer e.waitForRunnable.Done()
		if err := r.Start(e.internalCtx); err != nil {
			e.errChan <- err
		}
	}()
}

func (e *Server) engageStopProcedure(stopComplete <-chan struct{}) error {
	var shutdownCancel context.CancelFunc
	if e.opts.gracefulShutdownTimeout > 0 {
		e.shutdownCtx, shutdownCancel = context.WithTimeout(
			context.Background(), e.opts.gracefulShutdownTimeout)
	} else {
		e.shutdownCtx, shutdownCancel = context.WithCancel(context.Background())
	}
	defer shutdownCancel()
	close(e.internalProceduresStop)
	e.internalCancel()

	go func() {
		for {
			select {
			case err, ok := <-e.errChan:
				if ok {
					e.logger.Error(err, "error received after stop sequence was engaged")
				}
			case <-stopComplete:
				return
			}
		}
	}()
	if e.opts.gracefulShutdownTimeout == 0 {
		return nil
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.waitForRunnableToEnd(shutdownCancel)
}

func (e *Server) waitForRunnableToEnd(shutdownCancel context.CancelFunc) error {
	go func() {
		e.waitForRunnable.Wait()
		shutdownCancel()
	}()
	<-e.shutdownCtx.Done()
	if err := e.shutdownCtx.Err(); err != nil && err != context.Canceled {
		return fmt.Errorf(
			"failed waiting for all runnables to end within grace period of %s: %w",
			e.opts.gracefulShutdownTimeout, err)
	}
	return nil
}
