package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/go-admin-team/go-admin-core/logger"
	"github.com/go-admin-team/go-admin-core/sdk"
	"github.com/go-admin-team/go-admin-core/sdk/pkg"
)

type loggerKey struct{}

// GetRequestLogger 获取上下文提供的日志
func GetRequestLogger(c *gin.Context) *logger.Helper {
	var log *logger.Helper
	l, ok := c.Get(pkg.LoggerKey)
	if ok {
		ok = false
		log, ok = l.(*logger.Helper)
		if ok {
			return log
		}
	}
	//如果没有在上下文中放入logger
	requestId := pkg.GenerateMsgIDFromContext(c)
	log = logger.NewHelper(sdk.Runtime.GetLogger()).WithFields(map[string]interface{}{
		strings.ToLower(pkg.TrafficKey): requestId,
	})
	return log
}

// SetRequestLogger 设置logger中间件
func SetRequestLogger(c *gin.Context) {
	requestId := pkg.GenerateMsgIDFromContext(c)
	log := logger.NewHelper(sdk.Runtime.GetLogger()).WithFields(map[string]interface{}{
		strings.ToLower(pkg.TrafficKey): requestId,
	})
	c.Set(pkg.LoggerKey, log)
}
