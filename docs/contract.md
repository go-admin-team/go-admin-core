# The Runtime contract

This document is for anyone who registers something into `sdk.Runtime`: an
application module in go-admin, a fork, or a downstream framework such as
go-admin-pro. It states what the runtime registries promise, when they stop
accepting registrations, and what the panic guard does and does not cover.

`sdk.Runtime` is a package-level singleton (`sdk/application.go`) of type
`runtime.Runtime`, implemented by `*runtime.Application`.

---

## 1. Two kinds of state, and they have different rules

`Application` holds two kinds of field, and the difference matters because one
of them is rewritten while the server is running.

| Kind | Fields | When it may be written |
|---|---|---|
| **Registration** | `before`, `appRouters` | Until the matching `Run*()` runs. After that, refused. |
| **Registration (unsealed)** | `middlewares`, `handler` | Startup, by convention. Not enforced - see below. |
| **Resource** | `dbs`, `casbins`, `cache`, `queue`, `memoryQueue`, `configs`, `crontab` | Startup **and at run time**. Reloading the configuration rewrites these deliberately. |
| **Single value** | `engine`, `defaultTenant`, `routers`, `app` | Startup, by convention. Not enforced - see below. |

Only `before` and `appRouters` are enforced, because only they have an
execution entry point that can define the moment startup is over. `engine`,
`handler` and `middlewares` have no such moment, and guessing one would mean
dropping registrations for the wrong reason: `app/demo/router/router.go` sets
the engine from *inside* an app router callback, which is precisely what
`RunAppRouters()` executes.

Everything in the table is guarded by the same mutex regardless. Not sealed is
not the same as not protected.

---

## 2. When to register

> Registration APIs must be called before the matching `Run*()`. A call made
> afterwards is dropped and reported at ERROR level.

`init()` is the easiest place to satisfy that - the Go specification runs all
package initialisation on one goroutine before `main`, so there is no
concurrency to think about - but it is not the only legal one. go-admin-pro
registers from `run()`, and that is fine: `run()` happens before anything calls
`RunAppRouters()`.

The older wording, "registration is only allowed in `init()`", is not used here
because nothing can check it and real code already violates it.

To find out programmatically whether registration is still open:

```go
if sdk.Runtime.BeforeSealed() { /* RunBefore has already run */ }
if sdk.Runtime.AppRoutersSealed() { /* RunAppRouters has already run */ }
```

---

## 3. Execution order

Callbacks run **in registration order**, and nothing is reordered. For code in
`init()` that means the import order of the packages decides it.

Order is only promised *within* one registry. If the host program has its own
registry beside core's - go-admin has a package-level `AppRouters` slice - the
relative order of the two is the host's business, and it documents it. Do not
rely on it from a module.

---

## 4. The panic guard

Every callback executed through `RunBefore()` or `RunAppRouters()` runs behind
a `recover()`. A panicking callback does not stop the ones after it and does not
take the process down.

The panic is always reported, with three things:

- **where it was registered** (`file:line` of the `SetXxx` call). The stack
  unwinds into core, so without this the log names the framework rather than
  the module that failed;
- **the panic value**;
- **the full stack**.

```
ERROR runtime: appRouters callback crm-router (/src/app/crm/router/router.go:24) panicked, continuing with the rest: runtime error: index out of range [0] with length 0
goroutine 1 [running]:
...
```

### Failure levels

Two levels, expressed through options on the `*With` variants:

```go
// Default: a panic is reported, the remaining callbacks still run.
sdk.Runtime.SetAppRouters(router.InitRouter)

// Named, for a log line that says what it was rather than only where it was.
sdk.Runtime.SetAppRoutersWith(router.InitRouter, runtime.WithName("crm-router"))

// The process must not start without this one.
sdk.Runtime.SetBeforeWith(checkLicence, runtime.WithFatal(), runtime.WithName("licence"))
```

`WithFatal()` reports the panic and then exits with status 1. **It is the
exception.** A module that declares itself fatal makes the entire host program
as fragile as itself, and the exit skips every deferred cleanup and any
graceful shutdown - so it only belongs on work that runs before the server
starts listening. Unless the program is useless without you, let the default
degrade instead.

`SetBefore` and `SetAppRouters` are exactly the no-option case of the `*With`
variants: same registration path, same guard, same defaults.

### What the guard does not cover

> The guard covers **synchronous** panics: a panic in the callback body, or in
> anything it calls directly. A panic on a goroutine the callback starts is
> outside it, and always will be.

`recover()` only works on the goroutine that is panicking. This shape - which
is how go-admin-pro registers its jobs - gets no protection from the framework
at all:

```go
sdk.Runtime.SetBefore(func() {
    go func() {
        defer func() { /* this recover is yours to write */ }()
        job.InitJob()
    }()
})
```

If you start a goroutine, its `recover()` is your responsibility. Saying "the
framework has a guard" and leaving this out would be a promise the framework
cannot keep.

A callback that recovers for itself needs no special treatment: the panic stops
inside it, the callback returns normally, and the framework's guard sees
nothing and logs nothing. Two layers cost nothing.

Two more exits leave the guard behind, and neither is a panic, so neither is
something `recover()` was ever going to catch.

**`os.Exit`** ends the process where it is called. No deferred function runs -
not the callback's own, not the guard's - so nothing is logged and the
remaining callbacks never start.

`logger.Fatal` and `logger.Fatalf` reach it, but only sometimes, which is worse
than reaching it always. Both open with `if !Level.Enabled(FatalLevel) { return }`:

- with fatal logging on, they write the line and `os.Exit(1)` - the process is
  gone, and every later callback with it;
- with it filtered out, they write nothing and **return normally**, so the
  callback carries on past the line its author expected to be the last one.

Which of the two you get is decided by logging configuration, in a place that
has nothing to do with the decision being made. That is the reason this
framework does not use them for the `WithFatal()` path either. Keep them out of
callbacks: return an error state, or register with `WithFatal()` and panic -
that path exits on purpose, at a level nothing can filter, and names the
callback that did it.

**`runtime.Goexit`** ends the goroutine and does run the deferred functions on
the way out, which makes it the more confusing of the two: the guard's `defer`
executes, `recover()` returns `nil` because nothing is panicking, and the guard
concludes the callback returned normally. It did not. `Run*()` never returns,
every later callback is skipped, and there is no log line anywhere saying why.
`testing.T.Fatal` is the most common way to reach this by accident - a helper
written for tests, reused inside a callback.

Both are silent in a way a panic is not. If a startup hook can fail, let it
panic: that is the one failure shape the guard can see, name, and survive.

---

## 5. Sealing

Each registry closes when its own `Run*()` runs - `RunBefore()` closes `before`
and nothing else. From then on:

- `SetBefore` / `SetBeforeWith` (respectively `SetAppRouters` /
  `SetAppRoutersWith`) drop the registration and log at ERROR level, naming the
  registration site. They do not panic: a module that registered too late
  should not be able to bring down the host program.
- `Run*()` may be called again; it is a no-op. Callbacks that have already run
  do not run twice.
- `BeforeSealed()` / `AppRoutersSealed()` report the state.

**Resource fields are never sealed.** Freezing them would break configuration
reload (section 7), and it would break it silently.

### Sealing is sticky, and `sdk.Runtime` is process-wide

This is the one thing that bites in tests. Once a test has run a registry, every
later test in the same binary loses its registrations to the seal - and only to
an ERROR log, so nothing fails, routes are just missing.

`sdk.Runtime` is a `var`, so replace it:

```go
func TestSomething(t *testing.T) {
    previous := sdk.Runtime
    t.Cleanup(func() { sdk.Runtime = previous })
    sdk.Runtime = runtime.NewConfig()

    // ... register, run, assert ...
}
```

Do this in any test that calls `RunBefore()` or `RunAppRouters()`, directly or
through a startup helper.

---

## 6. `Get*` and self-written loops

`GetBefore()` and `GetAppRouters()` still work and still return
`[]func()` in registration order. They are **deprecated** in favour of
`RunBefore()` / `RunAppRouters()`, for two reasons: a loop you write yourself
gets no panic guard and no failure levels, and core cannot see it.

Two consequences to know about:

1. **They return a copy**, not the live slice. The caller ranges over the
   result with no lock held, and the registry can be written meanwhile;
   returning the field itself was the same defect `GetAllDb` was fixed for.
   Writing to the returned slice no longer reaches core's state.
2. **Mixing the two styles runs the callbacks twice.** Core cannot prevent it -
   it never sees a loop written downstream - so it warns instead:

   ```
   WARN runtime: GetBefore was called before RunBefore, and core cannot see a loop
   written outside it; if you run the returned callbacks yourself they run twice -
   drop your own loop
   ```

   Pick one style. `Run*()` is the one that is maintained.

---

## 7. Resources are rewritten at run time. This is by design.

Configuration is watched. When the file changes, core re-runs **every** setup
callback, while HTTP requests are being served:

```
config file changes
  → config/default.go   Entity.OnChange()
  → sdk/config/config.go  init() → runCallback()
  → your setup callbacks → SetDbByTenant / SetCasbinByTenant
                         → SetCacheAdapter / SetQueueAdapter
```

So:

- a resource setter being called again after startup is normal, not misuse;
- **the resource you are holding can be replaced while you hold it.** The lock
  makes a single access safe; it does not give you a consistent view across
  several. If a request needs the database and the cache to agree, fetch them
  once at the top and use those values, rather than calling the getters twice;
- your setup callback must be safe to run more than once.

---

## 8. Compatibility

`runtime.Runtime` is **only ever added to**. No existing method changes its
signature or its meaning, because the interface may be implemented or asserted
downstream.

Adding a method is still a source-breaking change for anyone who wrote their
own implementation of `runtime.Runtime` - a mock, typically. There are none in
go-admin, go-admin-pro or core itself, where the only implementation is
`*Application`; if you have one, add the new methods listed in section 2 and 4.
