package captcha

import (
	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/mojocn/base64Captcha"
)

type cacheStore struct {
	cache      storage.AdapterCache
	expiration int
}

// NewCacheStore returns a new standard memory store for captchas with the
// given collection threshold and expiration time (duration). The returned
// store must be registered with SetCustomStore to replace the default one.
func NewCacheStore(cache storage.AdapterCache, expiration int) base64Captcha.Store {
	s := new(cacheStore)
	s.cache = cache
	s.expiration = expiration
	return s
}

// Set sets the digits for the captcha id.
func (e *cacheStore) Set(id string, value string) error {
	return e.cache.Set(id, value, e.expiration)
}

// Get returns stored digits for the captcha id. Clear indicates
// whether the captcha must be deleted from the store.
func (e *cacheStore) Get(id string, clear bool) string {
	v, err := e.cache.Get(id)
	if err == nil {
		if clear {
			_ = e.cache.Del(id)
		}
		return v
	}
	return ""
}

// Verify captcha's answer directly
func (e *cacheStore) Verify(id, answer string, clear bool) bool {
	return e.Get(id, clear) == answer
}
