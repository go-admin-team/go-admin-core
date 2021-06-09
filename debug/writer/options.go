/*
 * @Author: lwnmengjing
 * @Date: 2021/6/3 8:33 上午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/3 8:33 上午
 */

package writer

// options 可配置参数
type options struct {
	path   string
	suffix string //文件扩展名
	cap    uint
}

func setDefault() options {
	return options{
		path:   "/tmp/go-admin",
		suffix: "log",
	}
}

type Option func(*options)

func WithPathOption(s string) Option {
	return func(o *options) {
		o.path = s
	}
}

func WithSuffixOption(s string) Option {
	return func(o *options) {
		o.suffix = s
	}
}

func WithCapOption(n uint) Option {
	return func(o *options) {
		o.cap = n
	}
}
