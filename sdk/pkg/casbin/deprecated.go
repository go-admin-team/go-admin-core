// Package mycasbin 已弃用
//
// Deprecated: 请使用 github.com/go-admin-team/go-admin-core/casbin 替代。
// 本包将在 v2.0.0 版本中移除。
//
// 迁移指南:
//   旧导入: import "github.com/go-admin-team/go-admin-core/sdk/pkg/casbin"
//   新导入: import "github.com/go-admin-team/go-admin-core/casbin"
package mycasbin

import (
	newcasbin "github.com/go-admin-team/go-admin-core/v2/casbin"
	"github.com/casbin/casbin/v3"
	"gorm.io/gorm"
)

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/casbin.Logger 替代
type Logger = newcasbin.Logger

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/casbin.Setup 替代
func Setup(db *gorm.DB, tableName string) *casbin.SyncedEnforcer {
	return newcasbin.Setup(db, tableName)
}
