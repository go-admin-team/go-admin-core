package config

import (
	"github.com/go-admin-team/go-admin-core/storage"
	"github.com/go-admin-team/go-admin-core/storage/locker"
	"github.com/redis/go-redis/v9"
)

var LockerConfig = new(Locker)

type Locker struct {
	Redis *RedisConnectOptions
}

// Empty 空设置
func (e Locker) Empty() bool {
	return e.Redis == nil
}

// Setup 启用顺序 redis > 其他 > memory
func (e Locker) Setup() (storage.AdapterLocker, error) {
	if e.Redis != nil {
		client := GetRedisClient()
		if client == nil {
			options, err := e.Redis.GetRedisOptions()
			if err != nil {
				return nil, err
			}
			client = redis.NewClient(options)
			_redis = client
		}
		return locker.NewRedis(client), nil
	}
	return nil, nil
}
