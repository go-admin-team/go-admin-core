// Package captcha 已弃用
//
// Deprecated: 请使用 github.com/go-admin-team/go-admin-core/captcha 替代。
// 本包将在 v2.0.0 版本中移除。
//
// 迁移指南:
//   旧导入: import "github.com/go-admin-team/go-admin-core/sdk/pkg/captcha"
//   新导入: import "github.com/go-admin-team/go-admin-core/captcha"
package captcha

import (
	newcaptcha "github.com/go-admin-team/go-admin-core/v2/captcha"
	"github.com/mojocn/base64Captcha"
)

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/captcha.SetStore 替代
func SetStore(s base64Captcha.Store) {
	newcaptcha.SetStore(s)
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/captcha.DriverStringFunc 替代
func DriverStringFunc() (id, b64s, answer string, err error) {
	return newcaptcha.DriverStringFunc()
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/captcha.DriverDigitFunc 替代
func DriverDigitFunc() (id, b64s, answer string, err error) {
	return newcaptcha.DriverDigitFunc()
}

// Deprecated: 使用 github.com/go-admin-team/go-admin-core/captcha.Verify 替代
func Verify(id, code string, clear bool) bool {
	return newcaptcha.Verify(id, code, clear)
}
