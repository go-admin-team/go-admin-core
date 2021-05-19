package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yahao333/go-admin-core/sdk/pkg"
)

// Error 失败数据处理
func Error(c *gin.Context, code int, err error, msg string) {
	var res response
	if err != nil {
		res.Msg = err.Error()
	}
	if msg != "" {
		res.Msg = msg
	}
	res.RequestId = pkg.GenerateMsgIDFromContext(c)
	res.Code = int32(code)
	c.Set("result", res)
	c.Set("status", code)
	c.AbortWithStatusJSON(http.StatusOK, res)
}

// OK 通常成功数据处理
func OK(c *gin.Context, data interface{}, msg string) {
	var res response
	res.Data = data
	if msg != "" {
		res.Msg = msg
	}
	res.RequestId = pkg.GenerateMsgIDFromContext(c)
	res.Code = http.StatusOK
	c.Set("result", res)
	c.Set("status", http.StatusOK)
	c.AbortWithStatusJSON(http.StatusOK, res)
}

// PageOK 分页数据处理
func PageOK(c *gin.Context, result interface{}, count int, pageIndex int, pageSize int, msg string) {
	var res page
	res.List = result
	res.Count = count
	res.PageIndex = pageIndex
	res.PageSize = pageSize
	OK(c, res, msg)
}

// Custum 兼容函数
func Custum(c *gin.Context, data gin.H) {
	data["requestId"] = pkg.GenerateMsgIDFromContext(c)
	c.Set("result", data)
	c.AbortWithStatusJSON(http.StatusOK, data)
}
