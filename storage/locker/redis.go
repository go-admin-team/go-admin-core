package locker

import (
	"context"
	"time"

	"github.com/go-redis/redis/v9"

	"github.com/bsm/redislock"

	"github.com/go-admin-team/go-admin-core/storage"
)

// NewRedis 初始化locker
func NewRedis(c *redis.Client) *Redis {
	return &Redis{
		client: c,
	}
}

var _ storage.AdapterLocker = &Redis{}

type Redis struct {
	client *redis.Client
	mutex  *redislock.Client
}

func (Redis) String() string {
	return "redis"
}

func (r *Redis) Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error) {
	if r.mutex == nil {
		r.mutex = redislock.New(r.client)
	}
	return r.mutex.Obtain(context.TODO(), key, time.Duration(ttl)*time.Second, options)
}
