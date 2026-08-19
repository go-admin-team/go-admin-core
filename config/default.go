package config

import (
	"bytes"
	"sync"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/config/loader"
	"github.com/go-admin-team/go-admin-core/v2/config/loader/memory"
	"github.com/go-admin-team/go-admin-core/v2/config/reader"
	"github.com/go-admin-team/go-admin-core/v2/config/reader/json"
	"github.com/go-admin-team/go-admin-core/v2/config/source"
	log "github.com/go-admin-team/go-admin-core/v2/logger"
)

type config struct {
	exit chan bool
	// exitOnce guards exit: the select-then-close in Close is a check followed
	// by an act, so two callers can both reach the close.
	exitOnce sync.Once
	opts     Options

	sync.RWMutex
	// the current snapshot
	snap *loader.Snapshot
	// the current values
	vals reader.Values
}

type watcher struct {
	lw    loader.Watcher
	rd    reader.Reader
	path  []string
	value reader.Value
}

func newConfig(opts ...Option) (Config, error) {
	var c config

	err := c.Init(opts...)
	if err != nil {
		return nil, err
	}
	go c.run()

	return &c, nil
}

func (c *config) Init(opts ...Option) error {
	c.opts = Options{
		Reader: json.NewReader(),
	}
	c.exit = make(chan bool)
	for _, o := range opts {
		o(&c.opts)
	}

	// default loader uses the configured reader
	if c.opts.Loader == nil {
		c.opts.Loader = memory.NewLoader(memory.WithReader(c.opts.Reader))
	}

	err := c.opts.Loader.Load(c.opts.Source...)
	if err != nil {
		return err
	}

	c.snap, err = c.opts.Loader.Snapshot()
	if err != nil {
		return err
	}

	c.vals, err = c.opts.Reader.Values(c.snap.ChangeSet)
	if err != nil {
		return err
	}
	if c.opts.Entity != nil {
		_ = c.vals.Scan(c.opts.Entity)
	}

	return nil
}

func (c *config) Options() Options {
	return c.opts
}

func (c *config) run() {
	watch := func(w loader.Watcher) error {
		for {
			// get changeset 获取变更集
			snap, err := w.Next()
			if err != nil {
				return err
			}

			c.Lock()

			if c.snap.Version >= snap.Version {
				c.Unlock()
				continue
			}

			// save 保存快照
			c.snap = snap

			// set values 设置值
			c.vals, err = c.opts.Reader.Values(snap.ChangeSet)
			if err != nil {
				c.Unlock()
				log.Errorf("failed to read values: %v", err)
				continue
			}
			if c.opts.Entity != nil {
				if err := c.vals.Scan(c.opts.Entity); err != nil {
					c.Unlock()
					log.Errorf("failed to scan entity: %v", err)
					continue
				}
				c.opts.Entity.OnChange()
			}

			c.Unlock()
		}
	}

	for {
		w, err := c.opts.Loader.Watch()
		if err != nil {
			log.Errorf("failed to start watcher: %v", err)
			// The exit check at the bottom sits past the point this jumps
			// back from, so without racing it here a loader that cannot be
			// watched keeps this goroutine retrying after Close.
			select {
			case <-c.exit:
				return
			case <-time.After(time.Second):
			}
			continue
		}

		done := make(chan bool)

		// the stop watch func 停止监控函数
		go func() {
			select {
			case <-done:
			case <-c.exit:
			}
			_ = w.Stop()
		}()

		// block watch 阻塞监控
		if err := watch(w); err != nil {
			// 记录错误日志
			log.Errorf("watch error: %v", err)
			select {
			case <-c.exit:
				close(done)
				return
			case <-time.After(time.Second):
			}
		}

		// Closed per iteration rather than deferred: a defer inside this loop
		// only runs when run returns, so every iteration left its stop-watch
		// goroutine alive until then.
		close(done)

		// if the config is closed exit 检查是否退出
		select {
		case <-c.exit:
			return
		default:
		}
	}
}

func (c *config) Map() map[string]interface{} {
	c.RLock()
	defer c.RUnlock()
	return c.vals.Map()
}

func (c *config) Scan(v interface{}) error {
	c.RLock()
	defer c.RUnlock()
	return c.vals.Scan(v)
}

// Sync sync loads all the sources, calls the parser and updates the config
func (c *config) Sync() error {
	if err := c.opts.Loader.Sync(); err != nil {
		return err
	}

	snap, err := c.opts.Loader.Snapshot()
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	c.snap = snap
	vals, err := c.opts.Reader.Values(snap.ChangeSet)
	if err != nil {
		return err
	}
	c.vals = vals

	return nil
}

func (c *config) Close() error {
	c.exitOnce.Do(func() { close(c.exit) })
	return nil
}

func (c *config) Get(path ...string) reader.Value {
	c.RLock()
	defer c.RUnlock()

	// did sync actually work?
	if c.vals != nil {
		return c.vals.Get(path...)
	}

	// no value
	return newValue()
}

func (c *config) Set(val interface{}, path ...string) {
	c.Lock()
	defer c.Unlock()

	if c.vals != nil {
		c.vals.Set(val, path...)
	}

	return
}

func (c *config) Del(path ...string) {
	c.Lock()
	defer c.Unlock()

	if c.vals != nil {
		c.vals.Del(path...)
	}

	return
}

func (c *config) Bytes() []byte {
	c.RLock()
	defer c.RUnlock()

	if c.vals == nil {
		return []byte{}
	}

	return c.vals.Bytes()
}

func (c *config) Load(sources ...source.Source) error {
	if err := c.opts.Loader.Load(sources...); err != nil {
		return err
	}

	snap, err := c.opts.Loader.Snapshot()
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	c.snap = snap
	vals, err := c.opts.Reader.Values(snap.ChangeSet)
	if err != nil {
		return err
	}
	c.vals = vals

	return nil
}

func (c *config) Watch(path ...string) (Watcher, error) {
	readerValue := c.Get(path...)

	w, err := c.opts.Loader.Watch(path...)
	if err != nil {
		return nil, err
	}

	return &watcher{
		lw:    w,
		rd:    c.opts.Reader,
		path:  path,
		value: readerValue,
	}, nil
}

func (c *config) String() string {
	return "config"
}

func (w *watcher) Next() (reader.Value, error) {
	for {
		s, err := w.lw.Next()
		if err != nil {
			return nil, err
		}

		// only process changes
		if bytes.Equal(w.value.Bytes(), s.ChangeSet.Data) {
			continue
		}

		v, err := w.rd.Values(s.ChangeSet)
		if err != nil {
			return nil, err
		}

		w.value = v.Get()
		return w.value, nil
	}
}

func (w *watcher) Stop() error {
	return w.lw.Stop()
}
