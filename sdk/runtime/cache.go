package runtime

import (
	"encoding/json"
	"time"

	"github.com/chanxuehong/wechat/oauth2"

	"github.com/go-admin-team/go-admin-core/storage"
)

const (
	intervalTenant = ""
)

// NewCache 创建对应上下文缓存
func NewCache(prefix string, store storage.AdapterCache, wxTokenStoreKey string) storage.AdapterCache {
	if wxTokenStoreKey == "" {
		wxTokenStoreKey = "wx_token_store_key"
	}
	return &Cache{
		prefix:          prefix,
		store:           store,
		wxTokenStoreKey: wxTokenStoreKey,
	}
}

type Cache struct {
	prefix          string
	store           storage.AdapterCache
	wxTokenStoreKey string
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
	return nil
	//return e.store.Connect()
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
