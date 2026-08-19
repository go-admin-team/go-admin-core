package cache_test

import (
	"testing"

	"github.com/go-admin-team/go-admin-core/v2/storage"
	"github.com/go-admin-team/go-admin-core/v2/storage/cache"
	"github.com/go-admin-team/go-admin-core/v2/storage/cachetest"
)

func TestMemCacheConformance(t *testing.T) {
	cachetest.Run(t, func(t *testing.T) storage.Cache {
		// No sweeper: the suite asserts lazy expiry, which must hold on its own.
		return cache.NewMemCacheWithSweep(0)
	})
}

func TestMemCacheWithSweeperConformance(t *testing.T) {
	cachetest.Run(t, func(t *testing.T) storage.Cache {
		return cache.NewMemCache()
	})
}
