package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/yahao333/go-admin-core/cache"
	"github.com/go-redis/redis/v7"
	"github.com/robinjoseph08/redisqueue/v2"
	"io/ioutil"
	"os"
	"time"
)

type cacheDriver string

const (
	MemoryCache cacheDriver = "memory"
	RedisCache  cacheDriver = "redis"
)

func (e cacheDriver) String() string {
	return string(e)
}

type Cache struct {
	Driver               cacheDriver //memory: 内存缓存，redis：redis缓存
	Network              string
	Addr                 string
	Username             string
	Password             string
	DB                   int
	PoolSize             int
	VisibilityTimeout    int
	BlockingTimeout      int
	ReclaimInterval      int
	BufferSize           int
	Concurrency          int
	ApproximateMaxLength bool
	Tls                  *Tls
}

type Tls struct {
	Cert string
	Key  string
	Ca   string
}

// CacheConfig cache配置
var CacheConfig = new(Cache)

func (e Cache) Setup() (cache.Adapter, error) {
	if e.Driver == "" {
		e.Driver = MemoryCache
	}
	var c cache.Adapter
	switch e.Driver {
	case MemoryCache:
		c = &cache.Memory{}
	case RedisCache:
		var t *tls.Config
		if e.Tls != nil && e.Tls.Cert != "" {
			// 从证书相关文件中读取和解析信息，得到证书公钥、密钥对
			cert, err := tls.LoadX509KeyPair(e.Tls.Cert, e.Tls.Key)
			if err != nil {
				fmt.Printf("tls.LoadX509KeyPair err: %v\n", err)
				os.Exit(-1)
			}
			// 创建一个新的、空的 CertPool，并尝试解析 PEM 编码的证书，解析成功会将其加到 CertPool 中
			certPool := x509.NewCertPool()
			ca, err := ioutil.ReadFile(e.Tls.Ca)
			if err != nil {
				fmt.Printf("ioutil.ReadFile err: %v\n", err)
				os.Exit(-1)
			}

			if ok := certPool.AppendCertsFromPEM(ca); !ok {
				fmt.Println("certPool.AppendCertsFromPEM err")
				os.Exit(-1)
			}
			t = &tls.Config{
				// 设置证书链，允许包含一个或多个
				Certificates: []tls.Certificate{cert},
				// 要求必须校验客户端的证书
				ClientAuth: tls.RequireAndVerifyClientCert,
				// 设置根证书的集合，校验方式使用 ClientAuth 中设定的模式
				ClientCAs: certPool,
			}
		}
		c = &cache.Redis{
			ConnectOption: &redis.Options{
				Network:   e.Network,
				Addr:      e.Addr,
				Username:  e.Username,
				Password:  e.Password,
				DB:        e.DB,
				PoolSize:  e.PoolSize,
				TLSConfig: t,
			},
			ConsumerOptions: &redisqueue.ConsumerOptions{
				VisibilityTimeout: time.Duration(e.VisibilityTimeout) * time.Second,
				BlockingTimeout:   time.Duration(e.BlockingTimeout) * time.Second,
				ReclaimInterval:   time.Duration(e.ReclaimInterval) * time.Second,
				BufferSize:        e.BufferSize,
				Concurrency:       e.Concurrency,
			},
			ProducerOptions: &redisqueue.ProducerOptions{
				StreamMaxLength:      int64(e.VisibilityTimeout),
				ApproximateMaxLength: e.ApproximateMaxLength,
			},
		}
	default:
		//没有配置，跳过
		return nil, errors.New("cache driver[] not support")
	}
	err := c.Connect()
	if err != nil {
		return nil, err
	}
	go c.Run()
	return c, err
}
