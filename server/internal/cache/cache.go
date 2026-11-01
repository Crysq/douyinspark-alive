// Package cache 封装 Redis：缓存、分布式锁、限流、幂等标记与事件广播。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache 是 Redis 的薄封装。
type Cache struct {
	rdb *redis.Client
}

// New 创建客户端并做一次 ping。
func New(addr, password string, db int) (*Cache, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db, DialTimeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Cache{rdb: rdb}, nil
}

// Client 暴露底层客户端（健康检查用）。
func (c *Cache) Client() *redis.Client { return c.rdb }

// Ping 检查可用性。
func (c *Cache) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// GetJSON 读取并反序列化缓存，未命中返回 false。
func (c *Cache) GetJSON(ctx context.Context, key string, out any) bool {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, out) == nil
}

// SetJSON 写入带 TTL 的 JSON 缓存。
func (c *Cache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.rdb.Set(ctx, key, raw, ttl)
}

// Del 删除缓存键。
func (c *Cache) Del(ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}
	c.rdb.Del(ctx, keys...)
}

// IncrWindow 固定窗口计数，返回当前窗口内的计数。
func (c *Cache) IncrWindow(ctx context.Context, key string, ttl time.Duration) int64 {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0
	}
	return incr.Val()
}

// Lock 获取分布式锁，返回是否成功与释放函数。
func (c *Cache) Lock(ctx context.Context, key string, ttl time.Duration) (bool, func()) {
	token := time.Now().UnixNano()
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil || !ok {
		return false, func() {}
	}
	release := func() {
		script := redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) end return 0`)
		script.Run(context.Background(), c.rdb, []string{key}, token)
	}
	return true, release
}

// MarkOnce 幂等标记：首次写入返回 true。
func (c *Cache) MarkOnce(ctx context.Context, key string, ttl time.Duration) bool {
	ok, err := c.rdb.SetNX(ctx, key, 1, ttl).Result()
	return err == nil && ok
}

// Publish 广播事件（多实例 SSE 同步）。
func (c *Cache) Publish(ctx context.Context, channel string, payload []byte) {
	c.rdb.Publish(ctx, channel, payload)
}

// Subscribe 订阅事件通道。
func (c *Cache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, channel)
}

// Blacklist 把 JWT 的 jti 加入黑名单（登出即时失效）。
func (c *Cache) Blacklist(ctx context.Context, jti string, ttl time.Duration) {
	if jti == "" {
		return
	}
	c.rdb.Set(ctx, "spark:jwt:black:"+jti, 1, ttl)
}

// IsBlacklisted 查询 jti 是否已失效。
func (c *Cache) IsBlacklisted(ctx context.Context, jti string) bool {
	if jti == "" {
		return false
	}
	n, err := c.rdb.Exists(ctx, "spark:jwt:black:"+jti).Result()
	return err == nil && n > 0
}

// Touch 记录账号心跳（在线状态，TTL 90s）。
func (c *Cache) Touch(ctx context.Context, key string) {
	c.rdb.Set(ctx, key, time.Now().Unix(), 90*time.Second)
}

// Alive 判断心跳是否存在。
func (c *Cache) Alive(ctx context.Context, key string) bool {
	n, err := c.rdb.Exists(ctx, key).Result()
	return err == nil && n > 0
}

// ErrMiss 表示缓存未命中。
var ErrMiss = errors.New("cache miss")
