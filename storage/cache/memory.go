package cache

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cast"
)

type item struct {
	Value   string
	Expired time.Time
}

// expired is shared by lazy deletion and the background sweep so the rule
// lives in one place.
func (i *item) expired(now time.Time) bool {
	return i.Expired.Before(now)
}

// defaultCleanupInterval is how often expired entries are swept.
const defaultCleanupInterval = time.Minute

// NewMemory returns a cache that sweeps expired entries every
// defaultCleanupInterval.
//
// Lazy deletion on Get is not enough to reclaim memory: a key that is written
// and never read again stays forever, so a long-running process only grows
// (issue #31).
//
// Call Close when the instance is not process-wide (repeated construction in
// tests, for example). Skipping it does not leak without bound — each instance
// owns exactly one goroutine.
func NewMemory() *Memory {
	return NewMemoryWithCleanupInterval(defaultCleanupInterval)
}

// NewMemoryWithCleanupInterval sets the sweep interval. A value of zero or
// less starts no sweeper, falling back to purely lazy expiry.
func NewMemoryWithCleanupInterval(interval time.Duration) *Memory {
	m := &Memory{
		items: new(sync.Map),
		stop:  make(chan struct{}),
	}
	if interval > 0 {
		m.wg.Add(1)
		go m.janitor(interval)
	}
	return m
}

type Memory struct {
	items *sync.Map
	mutex sync.RWMutex

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// janitor sweeps expired entries until Close is called.
func (m *Memory) janitor(interval time.Duration) {
	defer m.wg.Done()

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			m.deleteExpired()
		case <-m.stop:
			return
		}
	}
}

// deleteExpired removes every expired entry. It reuses the predicate and the
// deletion path of lazy expiry, so it drops exactly the entries the next read
// would have dropped and observable behaviour is unchanged.
func (m *Memory) deleteExpired() {
	now := time.Now()
	m.items.Range(func(k, v interface{}) bool {
		it, ok := v.(*item)
		if !ok || !it.expired(now) {
			return true
		}
		if key, ok := k.(string); ok {
			_ = m.del(key)
		}
		return true
	})
}

// Close stops the sweeper. It is safe to call more than once.
//
// It is deliberately not part of storage.AdapterCache, to avoid breaking
// existing implementations; holders of the interface can reach it by assertion:
//
//	if c, ok := cache.(interface{ Close() error }); ok {
//	    _ = c.Close()
//	}
func (m *Memory) Close() error {
	m.stopOnce.Do(func() {
		close(m.stop)
	})
	m.wg.Wait()
	return nil
}

func (*Memory) String() string {
	return "memory"
}

func (m *Memory) Get(key string) (string, error) {
	item, err := m.getItem(key)
	if err != nil || item == nil {
		return "", err
	}
	return item.Value, nil
}

func (m *Memory) getItem(key string) (*item, error) {
	var err error
	i, ok := m.items.Load(key)
	if !ok {
		return nil, nil
	}
	switch item := i.(type) {
	case *item:
		if item.expired(time.Now()) {
			//过期
			_ = m.del(key)
			//过期后删除，返回错误表示 key 不存在
			return nil, fmt.Errorf("key %s expired", key)
		}
		return item, nil
	default:
		err = fmt.Errorf("value of %s type error", key)
		return nil, err
	}
}

func (m *Memory) Set(key string, val interface{}, expire int) error {
	s, err := cast.ToStringE(val)
	if err != nil {
		return err
	}
	item := &item{
		Value:   s,
		Expired: time.Now().Add(time.Duration(expire) * time.Second),
	}
	return m.setItem(key, item)
}

func (m *Memory) setItem(key string, item *item) error {
	m.items.Store(key, item)
	return nil
}

func (m *Memory) Del(key string) error {
	return m.del(key)
}

func (m *Memory) del(key string) error {
	m.items.Delete(key)
	return nil
}

func (m *Memory) HashGet(hk, key string) (string, error) {
	item, err := m.getItem(hk + key)
	if err != nil || item == nil {
		return "", err
	}
	return item.Value, err
}

func (m *Memory) HashDel(hk, key string) error {
	return m.del(hk + key)
}

func (m *Memory) Increase(key string) error {
	return m.calculate(key, 1)
}

func (m *Memory) Decrease(key string) error {
	return m.calculate(key, -1)
}

func (m *Memory) calculate(key string, num int) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	item, err := m.getItem(key)
	if err != nil {
		return err
	}

	if item == nil {
		err = fmt.Errorf("%s not exist", key)
		return err
	}
	var n int
	n, err = cast.ToIntE(item.Value)
	if err != nil {
		return err
	}
	n += num
	item.Value = strconv.Itoa(n)
	return m.setItem(key, item)
}

func (m *Memory) Expire(key string, dur time.Duration) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	item, err := m.getItem(key)
	if err != nil {
		return err
	}
	if item == nil {
		err = fmt.Errorf("%s not exist", key)
		return err
	}
	item.Expired = time.Now().Add(dur)
	return m.setItem(key, item)
}
