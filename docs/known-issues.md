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

## 2. A hot reload leaks the previous queue's consumers

`storage/queue/memory.go`'s `Shutdown` sets a flag and releases its wait
group. It does not close the per-stream channels, and every consumer started
by `Register` is blocked in `for message := range q` on one of them, so none
of them ever returns. A host that rebuilds its queue adapter on every
configuration reload leaks one goroutine per consumer each time - measured at
three per reload for go-admin, roughly 25KB with the buffers.

It leaks; it does not lose messages. Producers fetch the current adapter per
call, so nothing is published to the abandoned channels after the swap.

The fix is not `close(q)`: `Append` publishes with
`select { case q <- m: default: }` and the consumer loop re-publishes a failed
message after sleeping, so closing underneath either one panics the process
during shutdown - worse than the leak. It needs a per-consumer stop channel,
and the re-publish path has to stop first. **Order: stop publishing, drain,
then close.**

## 3. Nothing can drain a queue on the way out

Neither `Memory.Shutdown` nor `MemQueue.Close` delivers what is still
buffered. `MemQueue.Close` sets `closed` before it signals its drain loop, and
the loop's `deliver` returns immediately when `closed` is set - so the drain
takes the messages out of the channel and discards them, which is not what the
comment above it describes.

Measured: with a consumer blocked and twenty messages published,
`Memory.Shutdown` delivers none of them and `MemQueue.Close` delivers one.
End to end, 227 requests produced zero `sys_opera_log` rows across a shutdown
that exited 0 and logged "Server exiting" - the successful path.

This is why section 12 of `contract.md` says a `BeforeExit` callback cannot
flush the queue: today there is nothing for it to call.

## 4. A reload can rebuild resources while the process is shutting down

The configuration watcher keeps running through shutdown, so an edit landing
in that window re-runs `database.Setup` and `storage.Setup` - rebuilding the
pool and the queue adapter as the process is being taken apart.
`BeginShutdown` closes the half that goes through the phase registry; closing
the watcher itself is the host's job, and `config.Close` returns without
waiting for a reload already in flight.

## 5. `logger.DefaultLogger` is written on every reload with no synchronisation

`Logger.Setup` assigns the package-level `logger.DefaultLogger` from the
watcher goroutine on each reload, while every log call in the process reads it
without a lock. An interface value is two words; a torn read is a crash, not a
stale line. It is a bare exported variable, so the fix is not local.

## 6. Only the first tenant gets a cron scheduler

`app/jobs`'s `Setup` loops over the tenant databases calling `setup`, and
`setup` ends in `select {}` and never returns. With more than one entry under
`databases:`, the loop never reaches the second one. The same `select {}` also
makes the `defer crontab.Stop()` above it unreachable, so the scheduler has
never been stopped on the way out either.
