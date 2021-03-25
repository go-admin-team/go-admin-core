package runtime

import (
	"encoding/json"
	"time"

	"github.com/bsm/redislock"
	"github.com/chanxuehong/wechat/oauth2"

	"github.com/go-admin-team/go-admin-core/cache"
)

const (
	intervalTenant = "/"
	prefixKey      = "__host"
)

type Cache struct {
	prefix          string
	store           cache.Adapter
	wxTokenStoreKey string
}

// NewCache 创建对应上下文缓存
func NewCache(prefix string, store cache.Adapter, wxTokenStoreKey string) cache.Adapter {
	if wxTokenStoreKey == "" {
		wxTokenStoreKey = "wx_token_store_key"
	}
	return &Cache{
		prefix:          prefix,
		store:           store,
		wxTokenStoreKey: wxTokenStoreKey,
	}
}

// String string输出
func (e *Cache) String() string {
	if e.store == nil {
		return ""
	}
	return e.store.String()
}

// SetPrefix 设置前缀
func (e *Cache) SetPrefix(prefix string) {
	e.prefix = prefix
}

// Connect 初始化
func (e Cache) Connect() error {
	return e.store.Connect()
}

// Get val in cache
func (e Cache) Get(key string) (string, error) {
	return e.store.Get(e.prefix + intervalTenant + key)
}

// Set val in cache
func (e Cache) Set(key string, val interface{}, expire int) error {
	return e.store.Set(e.prefix+intervalTenant+key, val, expire)
}

// Del delete key in cache
func (e Cache) Del(key string) error {
	return e.store.Del(e.prefix + intervalTenant + key)
}

// HashGet get val in hashtable cache
func (e Cache) HashGet(hk, key string) (string, error) {
	return e.store.HashGet(hk, e.prefix+intervalTenant+key)
}

// HashDel delete one key:value pair in hashtable cache
func (e Cache) HashDel(hk, key string) error {
	return e.store.HashDel(hk, e.prefix+intervalTenant+key)
}

// Increase value
func (e Cache) Increase(key string) error {
	return e.store.Increase(e.prefix + intervalTenant + key)
}

func (e Cache) Decrease(key string) error {
	return e.store.Decrease(e.prefix + intervalTenant + key)
}

func (e Cache) Expire(key string, dur time.Duration) error {
	return e.store.Expire(e.prefix+intervalTenant+key, dur)
}

// Token 获取微信oauth2 token
func (e Cache) Token() (token *oauth2.Token, err error) {
	var str string
	str, err = e.store.Get(e.prefix + intervalTenant + e.wxTokenStoreKey)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(str), token)
	return
}

// PutToken 设置微信oauth2 token
func (e Cache) PutToken(token *oauth2.Token) error {
	rb, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return e.store.Set(e.prefix+intervalTenant+e.wxTokenStoreKey, string(rb), int(token.ExpiresIn)-200)
}

// Register 注册消费者
func (e Cache) Register(name string, f cache.ConsumerFunc) {
	e.store.Register(name, f)
}

// Append 增加数据到生产者
func (e Cache) Append(message cache.Message) error {
	values := message.GetValues()
	if values == nil {
		values = make(map[string]interface{})
	}
	values[prefixKey] = e.prefix
	return e.store.Append(message)
}

// Run 运行
func (e Cache) Run() {
	e.store.Run()
}

// Shutdown 停止
func (e Cache) Shutdown() {
	e.store.Shutdown()
}

// Lock 返回分布式锁对象
func (e Cache) Lock(key string, ttl int64, options *redislock.Options) (*redislock.Lock, error) {
	return e.store.Lock(e.prefix+intervalTenant+key, ttl, options)
}
