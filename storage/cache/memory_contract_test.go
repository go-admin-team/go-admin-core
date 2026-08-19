package cache

import (
	"strings"
	"testing"
	"time"
)

// Memory is what Setup hands back when no redis is configured, which makes it
// the implementation most deployments actually run. Its answers to "the value
// is not here" are not uniform — a key that was never set is not an error, a
// key that expired is — and sdk/config keeps this implementation on that path
// precisely because call sites were written against those answers. Until now
// that reasoning lived in a comment and nothing enforced it.
func TestMemoryDistinguishesAbsentFromExpired(t *testing.T) {
	m := NewMemory()
	t.Cleanup(func() { _ = m.Close() })

	t.Run("a key that was never set is empty and not an error", func(t *testing.T) {
		got, err := m.Get("never-set")
		if err != nil {
			t.Errorf("Get on an absent key returned %v, want nil", err)
		}
		if got != "" {
			t.Errorf("Get on an absent key returned %q, want empty", got)
		}
	})

	t.Run("a key that expired is an error", func(t *testing.T) {
		if err := m.Set("short", "v", 1); err != nil {
			t.Fatalf("Set: %v", err)
		}
		time.Sleep(1100 * time.Millisecond)

		got, err := m.Get("short")
		if err == nil {
			t.Fatal("Get on an expired key returned no error; absent and expired are supposed to differ here")
		}
		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("error is %q, want it to say the key expired", err)
		}
		if got != "" {
			t.Errorf("Get on an expired key returned %q, want empty", got)
		}
	})

	t.Run("an expired key is gone afterwards, not still expiring", func(t *testing.T) {
		if err := m.Set("gone", "v", 1); err != nil {
			t.Fatalf("Set: %v", err)
		}
		time.Sleep(1100 * time.Millisecond)

		if _, err := m.Get("gone"); err == nil {
			t.Fatal("first Get after expiry should report the expiry")
		}
		// The read deletes it, so the second call sees an absent key and the
		// answer changes. Callers polling a key get an error exactly once.
		if _, err := m.Get("gone"); err != nil {
			t.Errorf("second Get returned %v, want nil: the expired entry should have been removed", err)
		}
	})

	// Recorded, not endorsed. Set stores now+expire seconds, so a zero expiry
	// is already in the past by the time anything reads it. Every other
	// implementation behind this interface — memcache, redis, and the
	// storage.Cache conformance suite's ZeroTTLNeverExpires — takes zero to
	// mean permanent, so the same call keeps its value with redis configured
	// and loses it without. Nothing passes zero today in this module or in
	// either consumer; this is here so that the next caller who tries finds a
	// test instead of a mystery.
	t.Run("a zero expiry expires at once, the opposite of every other implementation", func(t *testing.T) {
		if err := m.Set("forever", "v", 0); err != nil {
			t.Fatalf("Set: %v", err)
		}
		time.Sleep(50 * time.Millisecond)

		got, err := m.Get("forever")
		if err == nil {
			t.Fatal("a zero expiry survived; if this implementation was aligned with the others, this test is what needs updating")
		}
		if got != "" {
			t.Errorf("Get returned %q, want empty", got)
		}
	})
}
