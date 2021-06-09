/*
 * @Author: lwnmengjing
 * @Date: 2021/6/7 5:54 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/7 5:54 下午
 */

package server

import "time"

type Option func(*options)

type options struct {
	gracefulShutdownTimeout time.Duration
}

func setDefaultOptions() options {
	return options{
		gracefulShutdownTimeout: 5 * time.Second,
	}
}
