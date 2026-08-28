package captcha

import (
	"image/color"
	"sync"

	"github.com/mojocn/base64Captcha"
)

// SetStore 设置store
func SetStore(s base64Captcha.Store) {
	base64Captcha.DefaultMemStore = s
}

// The drivers are built once and shared.
//
// ConvertFonts reads the font files on every call, and the string driver was
// calling it per captcha: eleven megabytes allocated to render one image that
// is a couple of kilobytes. A converted driver is read-only afterwards - it
// holds the parsed fonts and the drawing parameters - so one instance serves
// every request.
//
// The Captcha wrapper is still built per call, because it binds the driver to
// whatever store SetStore has installed, and that can change.
var (
	stringDriverOnce sync.Once
	stringDriver     *base64Captcha.DriverString

	digitDriverOnce sync.Once
	digitDriver     *base64Captcha.DriverDigit
)

func DriverStringFunc() (id, b64s, answer string, err error) {
	stringDriverOnce.Do(func() {
		stringDriver = base64Captcha.NewDriverString(
			46, 140, 2, 2, 4,
			"234567890abcdefghjkmnpqrstuvwxyz",
			&color.RGBA{240, 240, 246, 246},
			nil,
			[]string{"wqy-microhei.ttc"},
		).ConvertFonts()
	})
	captcha := base64Captcha.NewCaptcha(stringDriver, base64Captcha.DefaultMemStore)
	return captcha.Generate()
}

func DriverDigitFunc() (id, b64s, answer string, err error) {
	digitDriverOnce.Do(func() {
		digitDriver = base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	})
	captcha := base64Captcha.NewCaptcha(digitDriver, base64Captcha.DefaultMemStore)
	return captcha.Generate()
}

// Verify 校验验证码
func Verify(id, code string, clear bool) bool {
	return base64Captcha.DefaultMemStore.Verify(id, code, clear)
}
