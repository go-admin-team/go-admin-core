# storage

Two contracts, `Cache` and `Queue`, each with an in-memory implementation for a
single instance and a Redis implementation for several.

Both contracts are enforced by a conformance suite in `storage/cachetest`. A
contract exercised by one implementation is not a contract, so every backend
runs the same cases.

## Why these exist

The previous `AdapterCache` and `AdapterQueue` interfaces had no context, no way
to tell a cache miss from a backend failure, and only in-memory implementations.
That last point is what blocked horizontal scaling: two instances of the same
application did not share a cache, and a message published on one was never seen
by the other.

The older interfaces still exist and still work. Nothing has to be migrated.

## Choosing a backend

The backend is a configuration decision, not a code one. **Redis is never
required.** With no `cache` or `queue` section the process runs entirely in
memory and needs nothing beside it, which is what a single instance should do:

```yaml
settings:
  # no cache section at all -> in-memory cache
  queue:
    memory:
      poolSize: 100
```

Add a `redis` section only when more than one instance has to share state:

```yaml
settings:
  cache:
    redis:
      addr: 127.0.0.1:6379
      password: ""
      db: 0
  queue:
    redis:
      addr: 127.0.0.1:6379
      db: 0
      group: go-admin          # the same for every instance of one application
      max_attempts: 3
      claim_min_idle_seconds: 30
```

`url` replaces the individual fields where a provider hands one out, and
`rediss://` selects TLS:

```yaml
  cache:
    redis:
      url: rediss://default:password@cache.example.com:6380/0
```

`config.CacheConfig.Setup()` and `config.QueueConfig.Setup()` read this and
return the adapter, in the order `redis > memory`. A `redis` section that
cannot be reached **fails the boot** rather than falling back to memory:
falling back would silently give an operator who asked for a shared cache one
that is not shared, and the symptom would only appear as strange behaviour
under load.

`Open()` returns the same backend through the current contract, for code that
wants a miss it can detect or a context it can cancel:

```go
c, err := config.CacheConfig.Open()   // storage.Cache
q, err := config.QueueConfig.Open()   // storage.Queue
```

### Switching a queue from memory to Redis

`Setup()`'s memory branch still returns the original in-process queue, so that
existing call sites keep the behaviour they were written against. It differs
from the Redis one in two ways that only show up on the switch:

- `Register` starts consuming by itself on memory. On Redis nothing is consumed
  until `Run` is called, so a call site that registers without running goes
  silently idle.
- The original memory queue retries a failed message three times on its own.
  Redis retries through `max_attempts`; `MemQueue`, which `Open()` returns, does
  not retry at all.
- **A full queue behaves differently.** The original memory queue drops the
  message and returns an error, so `poolSize` is the point at which messages
  start being lost rather than a tuning knob: one consumer goroutine per topic
  draining into a database will not keep up with a busy endpoint, and the
  default of 100 empties into losses under load. `MemQueue` and Redis apply
  back pressure instead — `Publish` blocks until there is room or the context
  is done. Size the original queue for the burst, or move to `Open()`.

Code that wants one behaviour on both backends should move to `Open()` and the
`Queue` contract.

Everything below describes the interfaces those adapters are built on, for code
that wants them directly.

## Cache

```go
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, val string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Incr(ctx context.Context, key string, delta int64) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	io.Closer
}
```

| Implementation | Constructor | Use |
| --- | --- | --- |
| memory | `cache.NewMemCache()` / `cache.NewMemCacheWithOptions(o)` | one instance, nothing to run |
| Redis | `redis.New(client)` / `redis.Open(ctx, url)` | several instances |

`redis.New` borrows the client and leaves it open on `Close`; `redis.Open` owns
its connection and closes it.

Rules worth knowing before writing against it:

- A miss is `ErrCacheMiss`, tested with `errors.Is`. Treating every error as
  "no data" turns a Redis outage into a full cache bypass and stampedes the
  database.
- An empty string is a legal value, returned with a nil error.
- A `ttl` of zero or less means no expiry. `Expire` with such a value makes an
  entry permanent rather than deleting it.
- `Incr` is atomic and starts an absent key from zero. The key it creates has
  no ttl.
- `ctx` is checked first, so a caller that gave up gets its own error rather
  than `ErrCacheClosed`.
- **`MemCache` is bounded — the older `Memory` is not.** `cache.NewMemCache`,
  which is what `Open()` returns, holds a million entries by default and evicts
  to stay within that, so an entry can disappear before its ttl expires. Expired
  entries are dropped first; only a shard with none to spare drops a live one.
  Treat any read as able to miss — which the contract already required.
  `cache.NewMemCacheWithOptions(cache.MemCacheOptions{MaxEntries: n})` sets
  another limit, and a negative `MaxEntries` restores unbounded growth, which is
  only safe where the keyspace is bounded by something else. Redis is bounded by
  that server's own `maxmemory` policy instead.

  `Setup()`'s memory branch still returns `cache.NewMemory`, which has **no such
  bound** and grows until the process runs out of memory. Everything reached
  through `AdapterCache` is on that path — `captcha.NewCacheStore` and
  `Runtime.SetCacheAdapter` among them — so a caller that writes one entry per
  request under a key it never reuses should either bound the keyspace itself or
  move to `Open()`.

`storage.WithPrefix(c, "app:")` namespaces one backend across applications.

### Adopting it without changing call sites

`config.CacheConfig.Setup()` already returns an `AdapterCache`, which is what
`captcha.NewCacheStore` and `Runtime.SetCacheAdapter` accept, so switching to
Redis is a configuration change and nothing else.

It does that through `storage.LegacyAdapter(c)`, which is also available
directly for a cache built by hand:

```go
c := cache.NewMemCache()          // or redis.New(client)
runtime.SetCacheAdapter(storage.LegacyAdapter(c))
```

What the adapter cannot do is repair the old contract: `Get` still reports a
miss as `("", nil)`, and the `Hash*` family still concatenates its arguments
into one flat key space. Move to `Cache` directly where either matters.

## Queue

```go
type Queue interface {
	Publish(ctx context.Context, msg Message) error
	Subscribe(topic string, h Handler) error
	Start(ctx context.Context) error
	io.Closer
}
```

| Implementation | Constructor | Use |
| --- | --- | --- |
| memory | `queue.NewMemQueue(size)` | one instance, nothing survives a restart |
| Redis streams | `redis.NewQueue(client, opts)` / `redis.OpenQueue(ctx, url, opts)` | several instances, messages survive a restart |

Order of operations: `Subscribe` every topic, then `Start`. `Start` blocks until
the context is done and reads only the topics registered when it was called.

- Publishing to a topic nobody consumes returns `ErrNoHandler` rather than
  dropping the message.
- Subscribing a topic twice returns `ErrTopicAlreadySubscribed`. The previous
  `Register` replaced the handler silently.
- A cancelled context is a clean exit: `Start` returns nil.
- `Message.Values` is transported as JSON, so only what JSON can express
  survives. A number arrives as a `float64`. Keep to strings where the exact
  type matters.

### Delivery semantics

Delivery is **at least once**. A handler must tolerate seeing the same message
twice — make it idempotent, or key it on `Message.ID`.

`Message.Attempts` counts deliveries and starts at 1.

| | memory | Redis |
| --- | --- | --- |
| survives a restart | no | yes |
| shared between instances | no | yes |
| handler error | message dropped after one attempt | retried up to `MaxAttempts` |
| instance disappears mid-handler | message lost | taken over by another instance |

The Redis implementation uses one consumer group per application. A message is
acknowledged only after its handler returns nil; a handler that returns an error
leaves the entry pending, and a later sweep claims it back and delivers it
again with `Attempts` incremented.

After `MaxAttempts` deliveries the message stops being retried but is
**deliberately not acknowledged**. It stays in the pending list, where an
operator can find it:

```
XPENDING <topic> <group>
XPENDING <topic> <group> - + 10
```

Acknowledging it there would have made an unhandled message look handled, which
is the failure mode this whole package is trying to remove. Inspect the entry,
fix the cause, then `XACK` or `XCLAIM` it by hand.

### QueueOptions

| Field | Settings key | Default | Notes |
| --- | --- | --- | --- |
| `Group` | `group` | `go-admin` | every instance of one application must share it; a different group gets its own copy of every message |
| `KeyPrefix` | `key_prefix` | none | prepended to the topic to form the stream key |
| `MaxAttempts` | `max_attempts` | 3 | deliveries before the message is left pending |
| `ClaimMinIdle` | `claim_min_idle_seconds` | 30s | idle time before another consumer may take a delivery over, and the interval between retry sweeps. **Set it above the running time of the slowest handler**, otherwise a slow handler's message is redelivered while it is still working |
| `Consumer` | — | hostname + pid | must be unique per instance |
| `Block` | — | 1s | how long one read waits before looping, and therefore how long `Start` takes to notice a cancellation |
| `Batch` | — | 16 | messages per read or sweep |

The fields with no settings key can only be set in code, through
`redis.NewQueue`. A key that is not listed here is ignored silently, as any
unknown key in a settings file is.

### Migrating from AdapterQueue

`storage.LegacyQueueAdapter(q)` presents a `Queue` as an `AdapterQueue`, and is
what `config.QueueConfig.Setup()` returns for a Redis queue. Existing call sites
keep working unchanged; `Register` before `Run`, as they already do.

Two things it cannot carry over:

- `Register` returns nothing, so a duplicate topic or an unreachable backend is
  logged rather than reported. The first handler stays in place; the older
  `Register` replaced it silently.
- `ConsumerFunc` takes no context, so a cancelled `Start` does not reach a
  handler that is already running.

Move a call site to `Queue` directly where either matters:

| Before | After |
| --- | --- |
| `Register(name, f)` | `Subscribe(topic, handler)`, before `Start`, error checked |
| `Append(msg)` | `Publish(ctx, storage.Message{Topic: …, Values: …})` |
| `Run()` | `Start(ctx)` in a goroutine, error checked |
| `Shutdown()` | `Close()`, or cancel the context passed to `Start` |
| `Messager` getters | `storage.Message` fields |
| `GetErrorCount()` | `Message.Attempts`, which counts deliveries from 1 rather than counting failures |

## Testing your own implementation

```go
func TestMyQueue(t *testing.T) {
	cachetest.RunQueue(t, func(t *testing.T) storage.Queue {
		return NewMyQueue()
	})
}
```

`cachetest.Run` does the same for `Cache`. The factory is called once per case
and must return a queue with an empty backing store.

The Redis suites skip themselves unless `REDIS_URL` is set:

```
REDIS_URL=redis://localhost:6379/0 go test -race ./storage/...
```

CI provides a Redis service container, so both implementations are checked
against the same cases on every push.
