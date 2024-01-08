/*
 * @Author: lwnmengjing
 * @Date: 2021/6/3 8:33 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/3 8:33 上午
 */

package writer

// Options 可配置参数
type Options struct {
	path       string //文件路径
	suffix     string //文件扩展名
	daysToKeep uint   //保存天数
	cap        uint   //文件大小
}

func setDefault() Options {
	return Options{
		path:       "/tmp/go-admin",
		suffix:     "log",
		daysToKeep: 7,
	}
}

// Option set options
type Option func(*Options)

// WithPath 设置文件路径
func WithPath(s string) Option {
	return func(o *Options) {
		o.path = s
	}
}

// WithSuffix 设置文件扩展名
func WithSuffix(s string) Option {
	return func(o *Options) {
		o.suffix = s
	}
}

// WithCap set cap
func WithCap(n uint) Option {
	return func(o *Options) {
		o.cap = n
	}
}

// WithDaysToKeep 设置文件保留天数
func WithDaysToKeep(n uint) Option {
	return func(o *Options) {
		o.daysToKeep = n
	}
}
