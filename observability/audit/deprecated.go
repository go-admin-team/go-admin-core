// Package log 已弃用
//
// Deprecated: 请使用 github.com/go-admin-team/go-admin-core/observe/audit 替代。
// 本包将在 v2.0.0 版本中移除。
//
// 迁移指南:
//   旧导入: import "github.com/go-admin-team/go-admin-core/observability/audit"
//   新导入: import "github.com/go-admin-team/go-admin-core/observe/audit"
package log

import (
	newaudit "github.com/go-admin-team/go-admin-core/v2/observe/audit"
	"time"
)

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Log 替代
type Log = newaudit.Log

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Record 替代
type Record = newaudit.Record

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Stream 替代
type Stream = newaudit.Stream

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.FormatFunc 替代
type FormatFunc = newaudit.FormatFunc

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Option 替代
type Option = newaudit.Option

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Options 替代
type Options = newaudit.Options

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.ReadOption 替代
type ReadOption = newaudit.ReadOption

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.ReadOptions 替代
type ReadOptions = newaudit.ReadOptions

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.TextFormat 替代
func TextFormat(r Record) string {
	return newaudit.TextFormat(r)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.JSONFormat 替代
func JSONFormat(r Record) string {
	return newaudit.JSONFormat(r)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Name 替代
func Name(n string) Option {
	return newaudit.Name(n)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Size 替代
func Size(s int) Option {
	return newaudit.Size(s)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Format 替代
func Format(f FormatFunc) Option {
	return newaudit.Format(f)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Since 替代
func Since(s time.Time) ReadOption {
	return newaudit.Since(s)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.Count 替代
func Count(c int) ReadOption {
	return newaudit.Count(c)
}

var (
	// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.DefaultSize 替代
	DefaultSize = newaudit.DefaultSize
	// Deprecated: 使用 github.com/go-admin-team/go-admin-core/observe/audit.DefaultFormat 替代
	DefaultFormat = newaudit.DefaultFormat
)
