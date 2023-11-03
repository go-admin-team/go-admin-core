module github.com/go-admin-team/go-admin-core/plugins/logger/zap

go 1.20

require (
	github.com/go-admin-team/go-admin-core v1.3.11
	go.uber.org/zap v1.26.0
)

require go.uber.org/multierr v1.10.0 // indirect

replace github.com/go-admin-team/go-admin-core v1.3.11 => ../../../
