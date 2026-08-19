// Package logger 已弃用
//
// Deprecated: 请使用 github.com/go-admin-team/go-admin-core/tools/gorm/gormlog 替代。
// 本包将在 v2.0.0 版本中移除。
//
// 迁移指南:
//   旧导入: import "github.com/go-admin-team/go-admin-core/tools/gorm/logger"
//   新导入: import "github.com/go-admin-team/go-admin-core/tools/gorm/gormlog"
//
// 或使用点导入:
//   旧: import . "github.com/go-admin-team/go-admin-core/tools/gorm/logger"
//   新: import . "github.com/go-admin-team/go-admin-core/tools/gorm/gormlog"
package logger

import (
	newgormlog "github.com/go-admin-team/go-admin-core/v2/tools/gorm/gormlog"
	"gorm.io/gorm/logger"
)

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/tools/gorm/gormlog.New 替代
func New(config logger.Config) logger.Interface {
	return newgormlog.New(config)
}
