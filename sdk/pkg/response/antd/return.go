package antd

import (
	"github.com/gin-gonic/gin"
	"github.com/go-admin-team/go-admin-core/sdk/pkg"
	"net/http"
)

// Error 失败数据处理
func Error(c *gin.Context, errCode string, errMsg string, showType string) {
	var res response
	res.Success = false
	if errMsg != "" {
		res.ErrorMessage = errMsg
	}
	if showType != "" {
		res.ShowType = showType
	}
	res.TraceId = pkg.GenerateMsgIDFromContext(c)
	res.ErrorCode = errCode
	c.Set("result", res)
	c.Set("status", errCode)
	c.AbortWithStatusJSON(http.StatusOK, res)
}

// OK 通常成功数据处理
func OK(c *gin.Context, data interface{}) {
	var res response
	res.Data = data
	res.Success = true
	res.TraceId = pkg.GenerateMsgIDFromContext(c)
	c.Set("result", res)
	c.Set("status", http.StatusOK)
	c.AbortWithStatusJSON(http.StatusOK, res)
}

// PageOK 分页数据处理
func PageOK(c *gin.Context, result interface{}, total int, current int, pageSize int) {
	var res pages
	res.List = result
	res.Total = total
	res.Current = current
	res.PageSize = pageSize
	OK(c, res)
}

// Custum 兼容函数
func Custum(c *gin.Context, data gin.H) {
	data["traceId"] = pkg.GenerateMsgIDFromContext(c)
	c.Set("result", data)
	c.AbortWithStatusJSON(http.StatusOK, data)
}
