# The Runtime contract

This document is for anyone who registers something into `sdk.Runtime`: an
application module, a fork, or a framework built on top of this one. It states
what the runtime registries promise, when they stop accepting registrations,
and what the panic guard does and does not cover.

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
concurrency to think about - but it is not the only legal one. Registering from
a `run()` that executes before `RunAppRouters()` is equally fine. The rule is
about ordering, not about which function you use.

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

`recover()` only works on the goroutine that is panicking. This shape - common
when a callback kicks off background work - gets no protection from the
framework at all:

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
this repository, where `*Application` is the only implementation; if you have
one, add the new methods listed in section 2 and 4.

---

## 9. Reusing the host's authentication and authorization middleware

An application that wants the same JWT and role checks the host itself uses
does not need to build its own or import the host package that built them.
The host registers three well-known keys with `SetMiddleware`
(`runtime.JwtTokenCheck`, `runtime.RoleCheck`, `runtime.PermissionCheck`), and
an application reads them back with `GetHandlerFunc`:

```go
jwtCheck, ok := sdk.Runtime.GetHandlerFunc(runtime.JwtTokenCheck)
if !ok {
    log.Fatal("JwtTokenCheck is not registered; is the host started via cmd/api?")
}
roleCheck, _ := sdk.Runtime.GetHandlerFunc(runtime.RoleCheck)
permCheck, _ := sdk.Runtime.GetHandlerFunc(runtime.PermissionCheck)

g := v1.Group("/order").Use(jwtCheck).Use(roleCheck).Use(permCheck)
```

`GetHandlerFunc` reports `ok=false` instead of panicking both when the key was
never registered and when it was registered with something other than a
`gin.HandlerFunc`. **Check `ok` and fail loudly if enforcement is not
optional** - a router that continues past a missing check, the way a bare type
assertion's panic tends to get swallowed by this framework's own panic guard
(section 4), turns a wiring mistake into silently unauthenticated routes.

**Requirement on the host**: all three keys must be registered as bound
`gin.HandlerFunc` closures (for example `authMiddleware.MiddlewareFunc()`),
never as an unbound method expression (`(*jwt.GinJWTMiddleware).MiddlewareFunc`)
- the latter has no receiver bound to it and cannot be turned into a working
handler no matter how a caller asserts its type. If the host builds a
separate JWT instance per module rather than one shared instance, this also
means the shared instance an application reads back is whichever one the host
happened to register last; a host that wants applications to get a
meaningful, single shared instance should construct it once, before
registering routes, rather than once per module.

---

## 10. Application-supplied menu and API entries

Package `sdk/contract/seed` lets an application ask to appear in the admin
UI - a sidebar entry, a button-level permission, an authorized API route -
without knowing what table any of that lives in. It defines `MenuSpec` and
`ApiSpec` (what an application can ask for) and `Seeder` (what the host
implements to turn those into rows), and nothing else: no `SysMenu`, no
`SysApi`, no table name, anywhere in core.

That is a deliberate, narrow boundary. Two versions of a menu row already
exist in this framework's typical host (one frozen at migration time, one
live), disagreeing on soft-delete shape, and the difference has caused real
bugs before a repository-local tool was built to catch it. A third copy in
core - the one package with no such tool watching it, because it ships to
every fork and every third-party application before any of their code even
exists - would recreate the same class of bug in the one place it is hardest
to fix later. So core carries the shape of the request (`MenuSpec`/`ApiSpec`)
and the host, through `RegisterSeeder`, keeps the schema knowledge.
`MenuSpec.Kind` takes the same `models.Directory`/`models.Menu`/`models.Button`
values `sdk/contract/models` already defines for `sys_menu.menu_type`, rather
than a second, identically-valued set of constants local to this package.

```go
func init() {
    // the host, once, e.g. from app/admin's own init()
    seed.RegisterSeeder(adminSeeder{})
}

// the application, from inside its own migration, inside its own transaction
err := seed.SeedMenus(tx, "order", []seed.MenuSpec{
    {Code: "root", Kind: models.Directory, Title: "Order"},
    {Code: "list", Parent: "root", Kind: models.Menu, Title: "Order list",
        Path: "/order", Component: "apps/order/index", ApiCodes: []string{"list"}},
}, []seed.ApiSpec{
    {Code: "list", Title: "order list", Path: "/api/v1/order", Method: "GET"},
})
```

**Read the security note on `Seeder` before assuming this is a sandbox - it is
not one.** `SeedMenus` runs with the same `*gorm.DB` the application's
migration already holds outside the call, so an application that wanted to
write `sys_menu`, `sys_api`, or `casbin_rule` directly, bypassing `Seeder`
entirely, always could - nothing in this package or in Go's type system stops
it. There is also an indirect route that never touches `casbin_rule` at all:
linking a menu to another application's (or the host's own) API through
`ApiCodes`, then waiting for an administrator to grant that menu to a role
through the ordinary admin UI, which generates the matching Casbin policy on
its own, attributed to the administrator's action rather than to the
application. **Installing an application means trusting it with the host's
database connection, at the same level of trust as importing any other Go
package into the binary.** `Seeder` exists so a well-behaved application does
not need to know the host's schema, not so a malicious one is contained.

**Requirements on the host implementing `Seeder`**:

- Populate all four tables a visible, working menu entry needs - `sys_api`,
  `sys_menu`, `sys_menu_api_rule`, and however role grants and Casbin
  policies get attached (`sys_role_menu`, `casbin_rule`) - not just the first
  two. A `Seeder` that only writes `sys_menu`/`sys_api` produces a menu no
  role can see and an API nothing authorizes, silently.
- Tag every row the `Seeder` writes with the `appCode` it was called with (for
  example an `app_code` column on `sys_menu` and `sys_api`), so that
  installing, auditing, or removing one application's contribution does not
  require guessing which rows are whose.
- Decide, and document, how ids are assigned across applications that did not
  coordinate with each other - `MenuSpec`/`ApiSpec` carry no id, on purpose;
  the host is the only party in a position to detect or prevent a collision.

---

## 11. Application configuration sections

`sdk/config.RegisterExtend[T any](key string) func() *T` lets a caller claim
one section of the `extend:` configuration tree and get back a function that
returns that section's most recently loaded value. Whatever the loaded
configuration has under `extend.<key>` is decoded into a fresh `*T`
independently of every other registered key, on every load and on every
reload triggered by the file watcher (section 7) - so the host and any number
of applications can each keep their own configuration without one
overwriting another's.

```go
type orderConfig struct {
    PaymentEndpoint string
    Timeout         int
}

var getOrderConfig = config.RegisterExtend[orderConfig]("order") // call from init(), same convention as SetAppRouters

func handler(c *gin.Context) {
    cfg := getOrderConfig() // safe from a request handler; see Concurrency below
    _ = cfg.PaymentEndpoint
}
```

```yaml
extend:
  order:
    PaymentEndpoint: https://payment.internal
    Timeout: 30
```

**Call `RegisterExtend` only from `init()`, before `Setup` runs** - the same
convention as `SetAppRouters` and `migration.ForApp`: registration relies on
Go's package-init ordering to be free of concurrent writers, not on a runtime
lock. **Registering the same key twice panics immediately** rather than
letting the second caller silently take over the first's section - unlike the
runtime registries in sections 1-8, there is no sealing moment here to refuse
a late registration against, so a duplicate key is caught the only way left:
at registration time, loudly. There is no `target` argument to pass `nil`
for: `RegisterExtend` allocates its own `T`, so that failure mode from an
earlier version of this API cannot occur any more.

**Concurrency, and why the accessor is safe without a lock of its own**: every
reload runs from the config watcher's own goroutine, for as long as the
process is running - a request handler reading the same section is
inherently concurrent with that. `RegisterExtend` closes that gap itself, by
never handing out a value it might later mutate: each reload decodes into a
brand-new `T` and atomically swaps a pointer to publish it, so the accessor
always returns a complete, self-consistent snapshot - never a half-old,
half-new value - and the caller's own struct never needs to implement
`json.Unmarshaler` or guard its fields with a mutex to be read safely from a
request path. It also never returns `nil`, even before the first load
completes: `T`'s zero value is published as soon as `RegisterExtend` returns,
so there is no nil check to forget on the one code path that runs before
`Setup` does its first `Scan`. What the caller must still get right is not
holding on to one snapshot across a call to the accessor - reading two
fields off the same returned `*T` is consistent; calling the accessor twice
and reading one field from each result is not, if a reload lands in between.

**Back-compat, not a replacement**: before `RegisterExtend` existed, the only
mechanism was pointing the package-level `config.ExtendConfig` at a struct and
letting the *entire* `extend:` section decode into it. That keeps working
completely unchanged for a host that has not adopted `RegisterExtend` at all -
including the fact that reading it concurrently with a reload is the host's
own problem to solve, exactly as before. It also keeps working *alongside*
`RegisterExtend` - a host that has not migrated still gets its data even
while an application registers its own section - with one caveat: an
unmigrated host's struct is handed the whole `extend:` tree, including every
application's section as unrecognised fields, which `encoding/json` silently
ignores unless a field name happens to collide. A host that wants a clean
separation, immune to that collision, should migrate to calling
`RegisterExtend` itself under a reserved key (for example `"__host__"`) for
its own section, and update its configuration files to nest its existing
`extend:` fields one level deeper under that key.

---

## 12. Life-cycle phases

A phase names a moment in the process's life that an application can hang
work on. There are four, and each is a permanent promise about what has
happened by the time it runs:

| Phase | Runs when | Available |
|---|---|---|
| `AfterResource` | the database, cache, queue and casbin are built | configuration, every resource |
| `BeforeRouter` | before the host installs its routes | the above, plus the engine |
| `AfterListen` | the socket is accepting connections | everything |
| `BeforeExit` | on the way out, after the server stopped serving | everything, while it is being taken down |

```go
sdk.Runtime.SetPhase(runtime.AfterResource, func() { /* ... */ })
sdk.Runtime.SetShutdown(func(ctx context.Context) { /* ... */ })
```

The numeric values are **not** part of the contract. They are spaced so a
phase can be added between two existing ones without moving what is already
there; do not persist them, transmit them, or compare them with `<`.

### `AfterResource` runs again, and its callbacks must tolerate that

Every other phase runs once and then refuses further registration.
`AfterResource` is different: it runs again after **every** configuration
reload, because a reload rebuilds the resources it is named for. A queue
consumer registered against the adapter that existed at startup is attached
to an adapter nobody publishes to any more; re-running is what re-attaches
it.

So a callback here has to be **idempotent with respect to the same
resource** - not "does nothing the second time". The queue example is the
reason for that wording: `Register` unconditionally starts a consumer
goroutine, and after a reload it *must*, because the adapter is new. What it
must not do is start a second consumer on an adapter it already handled.

The rule is therefore to remember the resource you last acted on and return
early when it has not changed - but **the getter is not where you get that
identity from**. `GetQueueAdapter` and `GetQueuePrefix` build a fresh `*Queue`
wrapper on every call, so comparing what they return compares two wrappers and
never matches; the same is true of any accessor that decorates what it hands
back. Record the resource where it is *created* - the setup callback that
built it knows which instance it installed - and compare against that.

Two consequences worth stating plainly:

- **`RunPhase(AfterResource)` is not synchronous.** Rounds are serialised: a
  call arriving while a round is running marks the phase as owing another
  round and returns immediately, without waiting. Only the once-only phases
  give the caller "your callbacks have finished" on return. A trigger is
  never dropped, because the reload that produced it may have swapped in a
  resource after the running round took its snapshot.
- **Do not call `RunPhase(AfterResource)` from inside an `AfterResource`
  callback.** It marks another round owing, from within the round, forever.
  There is no guard against it, deliberately: a guard would have to tell
  self-recursion apart from a genuine reload on another goroutine, and
  dropping the latter is exactly the bug this mechanism exists to prevent.

The registry is never closed, so it only ever grows. A callback count that
increases between rounds is reported as a warning - that is the one automatic
signal that something is registering on every reload instead of being
idempotent.

### `BeforeExit` runs in reverse, and is the one phase shutdown does not close

Cleanup runs **last-registered-first**. That is the right default for a chain
of hooks that built on one another; it says nothing about resources the host
created for itself, which this chain did not build and does not unwind.

`SetShutdown` is the context-taking half of the same phase - both register
into it, and one pass runs both. The context carries the host's shutdown
budget so a callback that can cut its work short has something to consult.

**What the context bounds is the wait, not the work.** When the budget is
gone `RunShutdown` stops waiting and returns `ctx.Err()`; the callback that
was running carries on until the process exits, possibly leaving a partial
write behind. Go cannot cancel a function that does not check for
cancellation, which is why callbacks are handed a context at all.

`WithFatal` is not honoured here. Exiting from inside cleanup skips every
callback after it, which is the opposite of what the option is for. The
callback is kept and the option is reported - refusing the registration
outright would mean one mistyped option silently costs you the cleanup.

### Once shutdown starts, the other phases stop

`BeginShutdown` marks the process as stopping. From then on `SetPhase` and
`RunPhase` do nothing for every phase except `BeforeExit`, and say so at
warning level. Without it, a configuration reload arriving in the shutdown
window re-runs `AfterResource` - rebuilding the pool and the adapter, and
re-registering consumers - undoing cleanup that has already run.

`BeforeExit` is exempt because it is the shutdown. Gating it would make the
flag skip the very work it was added to protect.

### What these phases cannot do

- **They cannot drain the queue.** `Memory.Shutdown` does not deliver what is
  still buffered, so a `BeforeExit` callback has nothing to call that would
  flush the log queue. See `known-issues.md`. A deployment that cannot lose
  audit rows must not rely on this phase for that.
- **They cannot take the process out of a load balancer.** The only hook here
  runs *after* the HTTP server stopped accepting, and Kubernetes expects
  readiness to start failing *before* that so endpoints are withdrawn first.
  There is no pre-shutdown hook yet; without one, a rolling update can still
  produce connection refused.
