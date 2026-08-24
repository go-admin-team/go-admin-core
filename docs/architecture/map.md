# The map

What each top-level directory is, which names collide, and what "verified"
means here. For whoever arrives without the history.

## Top level

| directory | what it is |
| --- | --- |
| `captcha` | base64Captcha wrapper: a store and the helpers around it |
| `casbin` | casbin v3 enforcer setup, and the logger it talks through |
| `config` | dynamic configuration, inherited from go-micro. Fifteen packages of loader / reader / encoder / source; consumers reach only `config/source/file`, through `NewConfig`. The rest is machinery with no callers outside this tree. |
| `errors` | error codes and the protobuf error type |
| `jwtauth` | JWT middleware, derived from gin-jwt. `jwtauth/user` reads claims off a context. |
| `logger` | the logging package: zap and logrus adapters, the async writer, sampling, redaction |
| `observe/audit` | audit records |
| `response` | gin response envelopes. `response/antd` is the shape the Ant Design front end expects. |
| `sdk` | the application layer. `sdk/runtime` holds `Runtime`, `sdk/config` holds the settings structs, `sdk/api` and `sdk/service` are the handler and service bases, `sdk/pkg` is what remains of an older grab bag. |
| `server` | a server manager and its listeners |
| `storage` | cache and queue **contracts**, their memory and redis implementations, and `cachetest` — the suite any implementation has to pass |
| `tools` | helpers that belong nowhere else, plus `coreupgrade`, the migration codemod |

## Names that collide

Five package names mean two different things each. The compiler will not help
you: picking the wrong one gives `undefined: X`, which reads like a missing
symbol rather than a wrong import.

| you write | you probably meant | the other one |
| --- | --- | --- |
| `package config` | `sdk/config` — settings structs, `CacheConfig`, `QueueConfig` | `config` — the dynamic loader |
| `package logger` | `logger` — the logging package | `sdk/pkg/logger` |
| `package memory` | `config/source/memory` — a config source | `config/loader/memory` — the loader |
| `package json` | `config/reader/json` | `config/encoder/json` |
| `package utils` | `tools/utils` | `sdk/pkg/utils` |

## What "verified" means

| command | what it guarantees | what it does not |
| --- | --- | --- |
| `make ci` | tidy, build, vet, staticcheck with no findings, the race detector across the whole tree with nothing excluded, and the exported API matching `api/core.txt` | anything about consumers |
| `scripts/canary.sh DIR` | a consumer, migrated by `coreupgrade`, compiles against this tree | that it runs — only that it builds |
| `scripts/check-language.sh origin/main` | no Chinese in added Go lines, or anywhere in a commit message — subject and body both | uncommitted work; it reads committed state |

## Traps

**Stage before you verify.** `api-check` is `git diff --exit-code api/`, which
compares the working tree against the index. Regenerating the snapshot and then
running `make ci` fails on your own change until you `git add` it.

**`check-language.sh` takes the base ref as an argument**, not as an
environment variable, and it compares committed state — unstaged work is
invisible to it.

**`go build ./...` writes a `coreupgrade` binary at the repository root.** It is
gitignored; it is still surprising.

**Twenty-nine of fifty-one packages have no tests.** `storage` is the exception
worth copying: a written contract plus `cachetest`, which every implementation
must pass. That suite is what caught the mistakes in the redis queue.

**`make lint` analyses the host platform and linux.** A file behind a build
tag is invisible to a run on any other platform: a finding in
`watcher_linux.go` survived every local run on a mac and failed in CI. The
staticcheck version is pinned for the same reason a gate should not change its
verdict on its own.

**Every lint exemption names its reason on the line above it.** There are
eleven, all either the compatibility layer referring to what it is compatible
with, generated protobuf code, or the working half of a feature nobody wired.
Adding one is a decision to be reviewed; `make lint` reports nothing otherwise.

**`Runtime` has fifty-seven methods and is reached through a package-level
global**, `sdk.Runtime`. Changes to it have effects no test in this repository
will catch; the canary is the only thing that will.

## Branches that are not lines of development

`main` carries v2. `release/v1.7` carries v1 and is an ancestor of `main`, not
a fork: nothing on it is missing from v2, so there is never anything to move in
that direction. Fixes flow one way, from `main` back to `release/v1.7`, and
only when they matter to someone who has not migrated.

Everything else has been deleted from the remote, having been read first. The
SHAs are here so any of them can be brought back with
`git branch <name> <sha>` — nothing below is reachable from `main`, so a git gc
will eventually collect them, and after that only a clone taken before this
commit still has them.

| branch | sha | last commit | what it was |
| --- | --- | --- | --- |
| `up` | `fbbcd03` | `2024-01-08` | Thirty-seven refactors and dependency bumps against the pre-v2 layout. Its one idea worth having — typed claim accessors, including one for identities beyond float64 — is in `jwtauth` now, arrived at independently four years later. Its own `claims.go` discards the result of an `fmt.Errorf`. |
| `dev` | `dbc1957` | `2026-08-19` | One commit, the `Get` timeout, which is on `main` as a separate change. |
| `fix/http-get-timeout` | `dbc1957` | `2026-08-19` | The same commit under another name. A private consumer pinned this SHA directly, which is why it ran with a fix neither released line had. |

`dev` and `fix/http-get-timeout` are the same commit under two names, which is
worth noticing before treating them as two pieces of unmerged work.

There was also a branch called `master`, `8a92061` from `2022-11-24`, carrying
the fix for the bare claim assertions that #136 arrived at independently four
years later. It is deleted too.

A branch that is one commit ahead of both `main` and `release/v1.7` is on
neither line. Ahead of `main` alone means it was cut from `main`; ahead of
`release/v1.7` alone means it predates the v2 split.
