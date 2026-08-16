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

## 2. Integration tests require a live MySQL

**Excluded from:** every target, via `testing.Short()` guards in the tests
themselves.

**Affected:** `tools/database.TestDBConfig_Init`

**Reproduce:** run `go test ./tools/database/` without `-short` while a MySQL
instance matching the hard-coded DSN is reachable.

**Plan:** move the DSN behind an environment variable so the test can run
against a CI service container instead of being skipped.
