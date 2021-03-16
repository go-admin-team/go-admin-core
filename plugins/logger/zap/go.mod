module github.com/go-admin-team/go-admin-core/plugins/logger/zap

go 1.14

require (
	github.com/go-admin-team/go-admin-core v1.3.0-rc.2
	go.uber.org/zap v1.16.0
)

replace (
	github.com/go-admin-team/go-admin-core => ../../../
)
