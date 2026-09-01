# Contributing

This project is consumed by several downstream applications and is moving
towards an international audience. The conventions below exist so that the
history, the source and the documentation stay readable to everyone.

## Language

**English is the default for everything that is committed**: commit messages,
code comments, identifiers, log messages, error strings and documentation.

Chinese is fine in issues, pull request discussion and in `README.zh-CN.md`.

Existing Chinese comments are not translated opportunistically — that is a
separate, tracked task. Do not add new ones.

## Commit messages

```
<type><emoji>: <summary in the imperative mood>

<body: why the change was needed, what breaks without it>
```

Types: `feat✨` `fix🐛` `refactor🎨` `perf⚡` `test✅` `docs📝` `build🔧`
`chore🔧` `ci👷`

- One semantic change per commit. Do not bundle unrelated edits.
- Stage explicit paths (`git add path/to/file`), never `git add -A`.
- The summary says what the commit does; the body says why it was necessary.
  A defect fix should state the failure it prevents.
- Do not add trailers naming tools or assistants.

## Code comments

Comments are for intent that the code cannot express. Prefer none over noise.

- Explain **why**, not **what**. If the code needs a narrator, simplify the code.
- Keep them short. Design background, reproduction steps and migration plans
  belong in `docs/`, with at most a one-line pointer from the source.
- Public API needs a doc comment; internal helpers usually do not.

## Locking in sdk/runtime

`Application` guards every field with one `sync.RWMutex`, and `sync.RWMutex` is
not reentrant. Four rules, each of which has already been paid for once:

- **Take `e.mux` exactly once per exported method.** Inside a locked section
  call only `xxxLocked` helpers.
- **A `xxxLocked` helper never takes the lock and never calls an exported
  method.** The suffix is the whole contract; keep it in the name.
- **Never run a user callback while holding the lock.** `Run*()` takes a
  snapshot, releases, and then executes. Callbacks routinely call back into
  `Application` - an app router asks for the engine and then sets it - and
  under the lock that deadlocks on the first one.
- **Never log while holding the lock.** `SetLogger` accepts any implementation,
  so logging runs code the application supplied. Decide inside the lock, print
  outside it.

`sdk/runtime/application_lock_test.go` guards the first two with a two-second
timeout per accessor; the deadlock it was written for hung thirteen of them.

## Documentation

| Content | Location |
|---|---|
| Defects CI tolerates, with reproduction steps | `docs/known-issues.md` |
| What the runtime registries promise downstream | `docs/contract.md` |
| Architecture decisions and migration plans | `docs/` |
| Usage and getting started | `README.md`, `README.zh-CN.md` |

## Branches

- `main` is the release line. `dev` is retired; `master` has been stale since 2022.
- Always branch off `main`; never commit to it directly.
- Verify with `git rev-list --count origin/A..origin/B` rather than trusting
  the repository's default-branch setting.

## Tests

- A defect fix ships with a test that fails without it. Verify this by
  reverting the fix and watching the test go red.
- Tests requiring external services must be guarded by `testing.Short()`.
- Never write into the repository from a test; use `t.TempDir()`.

## Before opening a pull request

```sh
make ci        # build + vet + race-enabled tests
make lint      # staticcheck
make vuln      # govulncheck
```

All must pass. If a test has to be excluded from `make test-race`, document it
in `docs/known-issues.md` with a reproduction command, an impact assessment and
a plan — and skip it by test name, never by package.
