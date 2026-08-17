package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cast"
)

// LegacyAdapter exposes a Cache through the older AdapterCache interface, so a
// new backend can be adopted without touching the call sites that still take
// AdapterCache — captcha.NewCacheStore and Runtime.SetCacheAdapter among them.
//
// It cannot repair the older contract, only preserve it:
//
//   - Get reports a miss as ("", nil), which is indistinguishable from a stored
//     empty string. Callers that need to tell them apart must move to Cache.
//   - HashGet and HashDel concatenate their arguments, matching the previous
//     in-memory behaviour. That shares one flat key space with ordinary keys,
//     so HashGet("a", "bc") and Set("abc", …) collide. The Hash family is not
//     carried over to Cache for this reason.
func LegacyAdapter(c Cache) AdapterCache {
	return &legacyAdapter{inner: c}
}

type legacyAdapter struct {
	inner Cache
}

// String reports the backend's own identity where it has one. Operators read
// this to tell what is actually running, so the bridge must not stand in front
// of it.
func (l *legacyAdapter) String() string {
	if s, ok := l.inner.(fmt.Stringer); ok {
		return s.String()
	}
	return "legacy"
}

func (l *legacyAdapter) Get(key string) (string, error) {
	v, err := l.inner.Get(context.Background(), key)
	if errors.Is(err, ErrCacheMiss) {
		return "", nil
	}
	return v, err
}

func (l *legacyAdapter) Set(key string, val interface{}, expire int) error {
	s, err := cast.ToStringE(val)
	if err != nil {
		return err
	}
	return l.inner.Set(context.Background(), key, s, time.Duration(expire)*time.Second)
}

func (l *legacyAdapter) Del(key string) error {
	return l.inner.Del(context.Background(), key)
}

func (l *legacyAdapter) HashGet(hk, key string) (string, error) {
	return l.Get(hk + key)
}

func (l *legacyAdapter) HashDel(hk, key string) error {
	return l.Del(hk + key)
}

func (l *legacyAdapter) Increase(key string) error {
	_, err := l.inner.Incr(context.Background(), key, 1)
	return err
}

func (l *legacyAdapter) Decrease(key string) error {
	_, err := l.inner.Incr(context.Background(), key, -1)
	return err
}

func (l *legacyAdapter) Expire(key string, dur time.Duration) error {
	err := l.inner.Expire(context.Background(), key, dur)
	if errors.Is(err, ErrCacheMiss) {
		// The older contract reported this as a plain error.
		return errors.New(key + " not exist")
	}
	return err
}
