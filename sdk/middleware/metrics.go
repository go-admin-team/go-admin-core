/*
 * @Author: lwnmengjing
 * @Date: 2021/6/9 5:57 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/9 5:57 下午
 */

package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
)

// Metrics returns a Gin measuring middleware.
func Metrics() gin.HandlerFunc {
	handlerID := ""
	m := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})
	return func(c *gin.Context) {
		r := &reporter{c: c}
		m.Measure(handlerID, r, func() {
			c.Next()
		})
	}
}

type reporter struct {
	c *gin.Context
}

func (r *reporter) Method() string { return r.c.Request.Method }

func (r *reporter) Context() context.Context { return r.c.Request.Context() }

func (r *reporter) URLPath() string {
	return r.c.FullPath()
}

func (r *reporter) StatusCode() int { return r.c.Writer.Status() }

func (r *reporter) BytesWritten() int64 { return int64(r.c.Writer.Size()) }
