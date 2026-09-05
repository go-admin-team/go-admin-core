// Package bootstrap wires the configuration to the life-cycle phases.
//
// It is a package of its own because of the direction of the imports, not
// because the code wants one. The trigger needs both sdk.Runtime and
// sdk/config, and neither of those can hold it: sdk cannot import sdk/config,
// because sdk/config's own test imports sdk and Go refuses the cycle that
// closes; sdk/config cannot import sdk, because sdk pulls in the whole of
// sdk/runtime - gin, casbin, gorm, cron - and sdk/config is what
// sdk/contract/actions depends on, which is the surface an application is
// meant to be able to use on its own. A leaf package that both of them can be
// imported by costs nothing to anybody who does not call it.
package bootstrap

import (
	"github.com/go-admin-team/go-admin-core/v2/config/source"
	"github.com/go-admin-team/go-admin-core/v2/sdk"
	"github.com/go-admin-team/go-admin-core/v2/sdk/config"
	"github.com/go-admin-team/go-admin-core/v2/sdk/runtime"
)

// SetupConfig loads the configuration, runs fs, and then announces that the
// resources they built are ready.
//
// It is config.Setup plus the AfterResource phase, and it exists so that the
// order is core's to keep rather than something every call site has to
// remember. The announcement goes last, after every callback in fs has run:
// the phase means "the database, the cache and the queue are ready", and a
// hook that gets it before the callbacks that build them is worse than no
// hook at all. Because fs is what config re-runs on a reload, the
// announcement repeats there too, which is what AfterResource is for.
//
// config.Setup itself deliberately does not announce anything. Only the
// command that serves traffic should: a migration or a config-dump command
// that started queue consumers and warmed caches on its way to doing
// something else would be doing the wrong thing quietly.
func SetupConfig(s source.Source, fs ...func()) {
	// Built rather than appended to: fs may share its backing array with a
	// slice the caller still holds, and append would write the trigger into
	// their spare capacity.
	callbacks := make([]func(), 0, len(fs)+1)
	callbacks = append(callbacks, fs...)
	callbacks = append(callbacks, func() {
		// Read at call time, not captured: a process that replaces
		// sdk.Runtime - which the tests do - should have the replacement
		// announced to, not the one that was there at wiring time.
		sdk.Runtime.RunPhase(runtime.AfterResource)
	})
	config.Setup(s, callbacks...)
}
