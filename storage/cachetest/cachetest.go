// Package cachetest provides a black-box conformance suite for
// storage.Cache implementations.
//
// Every implementation must pass the same suite. A contract exercised by only
// one implementation is not a contract: the previous AdapterCache was shaped
// around Redis, its Redis implementation was removed, and the remaining
// in-memory one emulated the shape with string concatenation — nothing checked
// that the two ever agreed.
package cachetest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/storage"
)

// Factory builds a Cache for a single test. Cleanup is the caller's business;
// the suite always calls Close.
type Factory func(t *testing.T) storage.Cache

// Run executes the full suite against the implementation built by newCache.
func Run(t *testing.T, newCache Factory) {
	t.Helper()

	tests := []struct {
		name string
		fn   func(*testing.T, Factory)
	}{
		{"MissIsDistinguishable", testMissIsDistinguishable},
		{"EmptyValueIsNotAMiss", testEmptyValueIsNotAMiss},
		{"ExpiredKeyReportsMiss", testExpiredKeyReportsMiss},
		{"ZeroTTLNeverExpires", testZeroTTLNeverExpires},
		{"SetOverwrites", testSetOverwrites},
		{"DelRemoves", testDelRemoves},
		{"DelAbsentIsNotAnError", testDelAbsentIsNotAnError},
		{"IncrFromAbsent", testIncrFromAbsent},
		{"IncrAccumulates", testIncrAccumulates},
		{"IncrIsAtomic", testIncrIsAtomic},
		{"ExpireOnAbsentReportsMiss", testExpireOnAbsentReportsMiss},
		{"ExpireShortensLifetime", testExpireShortensLifetime},
		{"ExpireZeroMakesPermanent", testExpireZeroMakesPermanent},
		{"CloseIsIdempotent", testCloseIsIdempotent},
		{"UseAfterCloseReportsClosed", testUseAfterCloseReportsClosed},
		{"ContextCancellationIsHonoured", testContextCancellationIsHonoured},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, newCache) })
	}
}

func ctx() context.Context { return context.Background() }

// The single most important property: a miss must be distinguishable from a
// backend failure, otherwise an outage silently becomes a full cache bypass.
func testMissIsDistinguishable(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	_, err := c.Get(ctx(), "absent")
	if !errors.Is(err, storage.ErrCacheMiss) {
		t.Fatalf("Get on an absent key: got %v, want ErrCacheMiss", err)
	}
}

func testEmptyValueIsNotAMiss(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	if err := c.Set(ctx(), "empty", "", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, err := c.Get(ctx(), "empty")
	if err != nil {
		t.Fatalf("an empty value must not read as a miss: %v", err)
	}
	if v != "" {
		t.Errorf("got %q, want an empty string", v)
	}
}

func testExpiredKeyReportsMiss(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	if err := c.Set(ctx(), "brief", "v", 30*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		_, err := c.Get(ctx(), "brief")
		return errors.Is(err, storage.ErrCacheMiss)
	}, "an expired key must report ErrCacheMiss, the same error as an absent key")
}

func testZeroTTLNeverExpires(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	if err := c.Set(ctx(), "permanent", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	v, err := c.Get(ctx(), "permanent")
	if err != nil {
		t.Fatalf("a ttl of zero must mean no expiry, got %v", err)
	}
	if v != "v" {
		t.Errorf("got %q, want %q", v, "v")
	}
}

func testSetOverwrites(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	_ = c.Set(ctx(), "k", "first", time.Minute)
	if err := c.Set(ctx(), "k", "second", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, err := c.Get(ctx(), "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "second" {
		t.Errorf("got %q, want %q", v, "second")
	}
}

func testDelRemoves(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	_ = c.Set(ctx(), "a", "1", time.Minute)
	_ = c.Set(ctx(), "b", "2", time.Minute)

	if err := c.Del(ctx(), "a", "b"); err != nil {
		t.Fatalf("Del: %v", err)
	}

	for _, k := range []string{"a", "b"} {
		if _, err := c.Get(ctx(), k); !errors.Is(err, storage.ErrCacheMiss) {
			t.Errorf("%q survived Del: %v", k, err)
		}
	}
}

func testDelAbsentIsNotAnError(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	if err := c.Del(ctx(), "never-existed"); err != nil {
		t.Errorf("deleting an absent key must not fail: %v", err)
	}
}

func testIncrFromAbsent(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	n, err := c.Incr(ctx(), "counter", 5)
	if err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if n != 5 {
		t.Errorf("an absent key must start from zero: got %d, want 5", n)
	}
}

func testIncrAccumulates(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	_, _ = c.Incr(ctx(), "counter", 10)
	n, err := c.Incr(ctx(), "counter", -3)
	if err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if n != 7 {
		t.Errorf("got %d, want 7", n)
	}
}

// Guards the defect the previous implementation shipped: its read-modify-write
// was not atomic, so 200 concurrent increments landed on 151.
func testIncrIsAtomic(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := c.Incr(ctx(), "atomic", 1); err != nil {
				t.Errorf("Incr: %v", err)
			}
		}()
	}
	wg.Wait()

	got, err := c.Incr(ctx(), "atomic", 0)
	if err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if got != n {
		t.Errorf("lost updates: got %d after %d concurrent increments", got, n)
	}
}

func testExpireOnAbsentReportsMiss(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	err := c.Expire(ctx(), "absent", time.Minute)
	if !errors.Is(err, storage.ErrCacheMiss) {
		t.Errorf("Expire on an absent key: got %v, want ErrCacheMiss", err)
	}
}

func testExpireShortensLifetime(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	_ = c.Set(ctx(), "k", "v", time.Hour)
	if err := c.Expire(ctx(), "k", 30*time.Millisecond); err != nil {
		t.Fatalf("Expire: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		_, err := c.Get(ctx(), "k")
		return errors.Is(err, storage.ErrCacheMiss)
	}, "Expire must shorten the lifetime")
}

func testExpireZeroMakesPermanent(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	_ = c.Set(ctx(), "k", "v", 40*time.Millisecond)
	if err := c.Expire(ctx(), "k", 0); err != nil {
		t.Fatalf("Expire: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	if _, err := c.Get(ctx(), "k"); err != nil {
		t.Errorf("a ttl of zero must clear the expiry, got %v", err)
	}
}

func testCloseIsIdempotent(t *testing.T, newCache Factory) {
	c := newCache(t)

	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close must be a no-op, got %v", err)
	}
}

func testUseAfterCloseReportsClosed(t *testing.T, newCache Factory) {
	c := newCache(t)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := c.Set(ctx(), "k", "v", time.Minute); !errors.Is(err, storage.ErrCacheClosed) {
		t.Errorf("Set after Close: got %v, want ErrCacheClosed", err)
	}
	if _, err := c.Get(ctx(), "k"); !errors.Is(err, storage.ErrCacheClosed) {
		t.Errorf("Get after Close: got %v, want ErrCacheClosed", err)
	}
}

func testContextCancellationIsHonoured(t *testing.T, newCache Factory) {
	c := newCache(t)
	defer c.Close()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Set(cancelled, "k", "v", time.Minute)
	if err == nil {
		t.Error("Set with a cancelled context must fail")
		return
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want an error wrapping context.Canceled", err)
	}
}

func waitFor(t *testing.T, limit time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error(strings.TrimSpace(msg))
}
