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

**Every lint exemption names its reason on the line above it.** There are
eleven, all either the compatibility layer referring to what it is compatible
with, generated protobuf code, or the working half of a feature nobody wired.
Adding one is a decision to be reviewed; `make lint` reports nothing otherwise.

**`Runtime` has fifty-seven methods and is reached through a package-level
global**, `sdk.Runtime`. Changes to it have effects no test in this repository
will catch; the canary is the only thing that will.
