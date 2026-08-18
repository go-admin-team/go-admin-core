# Known Issues

Issues that CI deliberately tolerates. Each entry states why it is excluded,
how to reproduce it, and when it is expected to be resolved. Nothing may be
added here without a reproduction command.

## 1. Integration tests require a live MySQL

**Excluded from:** every target, via `testing.Short()` guards in the tests
themselves.

**Affected:** `tools/database.TestDBConfig_Init`

**Reproduce:** run `go test ./tools/database/` without `-short` while a MySQL
instance matching the hard-coded DSN is reachable.

**Plan:** move the DSN behind an environment variable so the test can run
against a CI service container instead of being skipped.
