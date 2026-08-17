package storage_test

import (
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/cache"
)

// The adapter must satisfy the older contract exactly, so existing call sites
// keep working when a new backend is put behind it.
func TestLegacyAdapterPreservesOldContract(t *testing.T) {
	a := storage.LegacyAdapter(cache.NewMemCacheWithSweep(0))

	if err := a.Set("k", "v", 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if v, err := a.Get("k"); err != nil || v != "v" {
		t.Fatalf("Get: got (%q, %v)", v, err)
	}

	// A miss is reported as an empty string with no error, as before.
	if v, err := a.Get("absent"); err != nil || v != "" {
		t.Errorf("miss: got (%q, %v), want (\"\", nil)", v, err)
	}

	// Set accepts non-string values through cast, as before.
	if err := a.Set("n", 42, 60); err != nil {
		t.Fatalf("Set(int): %v", err)
	}
	if err := a.Increase("n"); err != nil {
		t.Fatalf("Increase: %v", err)
	}
	if v, _ := a.Get("n"); v != "43" {
		t.Errorf("Increase: got %q, want %q", v, "43")
	}
	if err := a.Decrease("n"); err != nil {
		t.Fatalf("Decrease: %v", err)
	}
	if v, _ := a.Get("n"); v != "42" {
		t.Errorf("Decrease: got %q, want %q", v, "42")
	}

	if err := a.Expire("k", time.Minute); err != nil {
		t.Errorf("Expire: %v", err)
	}
	if err := a.Expire("absent", time.Minute); err == nil {
		t.Error("Expire on an absent key must fail, as before")
	}

	if err := a.Del("k"); err != nil {
		t.Errorf("Del: %v", err)
	}
}

// captcha.NewCacheStore takes an AdapterCache; the adapter must fit it.
func TestLegacyAdapterSatisfiesAdapterCache(t *testing.T) {
	var _ storage.AdapterCache = storage.LegacyAdapter(cache.NewMemCacheWithSweep(0))
}
