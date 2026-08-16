# Known Issues

Issues that CI deliberately tolerates. Each entry states why it is excluded,
how to reproduce it, and when it is expected to be resolved. Nothing may be
added here without a reproduction command.

## 1. Data race in `config/loader/memory`

**Excluded from:** `make test-race` (via `RACE_SKIP`)

**Reproduce:**

```sh
go test -race ./config/ -run TestConfigWatcherDirtyOverrite
```

**What happens:** `(*watcher).Next` writes a field that `(*memory).update`
reads concurrently, with no synchronisation between them.

```
Write at 0x... by goroutine 9:
  config/loader/memory.(*watcher).Next()   memory.go:404
Previous read at 0x... by goroutine 8:
  config/loader/memory.(*memory).update()
```

**Impact:** limited to the dynamic config watcher. All consumers only use
`config/source/file` and none of them subscribe to change events, so no
production code path reaches it today.

**Plan:** the `config` package is scheduled to be reduced to a thin loader —
the four-layer `loader`/`reader`/`encoder`/`secrets` stack has no users. This
race disappears with that work rather than being patched in place.

## 2. The open-source go-admin does not build against this tree

**Excluded from:** the canary run — it is expected to fail until the version
unification work lands.

**Reproduce:**

```sh
scripts/canary.sh /path/to/go-admin
```

**Status** (measured 2026-08-17): every consumer already on the merged-module
layout builds clean. The open-source `go-admin`, still pinned to `v1.5.3-rc`,
does not.

**Why it fails** — four independent causes, in the order the compiler reports
them:

1. It still requires the separate `go-admin-core/sdk` module, which no longer
   exists after the module merge.
2. `captcha.NewCacheStore` moved when `sdk/pkg/captcha` was promoted to a
   top-level package.
3. `GetCasbinKey` and `GetCrontabKey` were renamed to `GetCasbinByTenant` and
   `GetCrontabByTenant`.
4. `SetCrontab(string, *cron.Cron)` became `SetCrontab(*cron.Cron)` — a
   same-name, different-signature change.

Beyond these, the consumer pins `casbin/v2` while this module is on
`casbin/v3`. `Runtime` exposes `*casbin.SyncedEnforcer` directly, and across a
major version that is a different type, so no deprecation shim can bridge it.

**Plan:** this is a planned breaking upgrade, not a regression. It needs a
codemod shipped alongside it. Note that cause 4 is the dangerous class: the
obvious fix is to drop the first argument, which lands on a method that used to
deadlock on call until it was repaired.

## 3. Integration tests require a live MySQL

**Excluded from:** every target, via `testing.Short()` guards in the tests
themselves.

**Affected:** `tools/database.TestDBConfig_Init`

**Reproduce:** run `go test ./tools/database/` without `-short` while a MySQL
instance matching the hard-coded DSN is reachable.

**Plan:** move the DSN behind an environment variable so the test can run
against a CI service container instead of being skipped.
