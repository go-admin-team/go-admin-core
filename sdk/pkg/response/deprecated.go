// Package response 已弃用
//
// Deprecated: 请使用 github.com/go-admin-team/go-admin-core/response 替代。
// 本包将在 v2.0.0 版本中移除。
//
// 迁移指南:
//   旧导入: import "github.com/go-admin-team/go-admin-core/sdk/pkg/response"
//   新导入: import "github.com/go-admin-team/go-admin-core/response"
package response

import (
	newresponse "github.com/go-admin-team/go-admin-core/v2/response"
	"github.com/gin-gonic/gin"
)

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/response.Error 替代
func Error(c *gin.Context, code int, err error, msg string) {
	newresponse.Error(c, code, err, msg)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/response.OK 替代
func OK(c *gin.Context, data interface{}, msg string) {
	newresponse.OK(c, data, msg)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/response.PageOK 替代
func PageOK(c *gin.Context, result interface{}, count int, pageIndex int, pageSize int, msg string) {
	newresponse.PageOK(c, result, count, pageIndex, pageSize, msg)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/response.Custum 替代
func Custum(c *gin.Context, data gin.H) {
	newresponse.Custum(c, data)
}
