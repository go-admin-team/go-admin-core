package config

type Jwt struct {
	Secret string
	// Timeout token 有效期，单位：秒
	Timeout int64
	// MaxRefresh 续期上限，单位：秒。
	//
	// 自 token 首次签发起计算，超过该时长后不再允许续期，必须重新登录。
	// 一个 token 的最长存活时间为 Timeout + MaxRefresh。
	//
	// 为 0 时由使用方决定默认值，本结构不做兜底。
	MaxRefresh int64
}

var JwtConfig = new(Jwt)
