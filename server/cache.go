package main

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Cache master plan: Redis di produksi (pre-loading data hot), in-memory utk
// dev lokal tanpa Redis.
// ponytail: satu interface 3 method; upgrade ke struktur kaya kalau butuh.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key, val string, ttl time.Duration)
	Del(ctx context.Context, keys ...string)
}

type memEntry struct {
	val string
	exp time.Time
}

type memCache struct {
	mu sync.RWMutex
	m  map[string]memEntry
}

func (c *memCache) Get(_ context.Context, key string) (string, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.exp) {
		return "", false
	}
	return e.val, true
}

func (c *memCache) Set(_ context.Context, key, val string, ttl time.Duration) {
	c.mu.Lock()
	c.m[key] = memEntry{val, time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *memCache) Del(_ context.Context, keys ...string) {
	c.mu.Lock()
	for _, k := range keys {
		delete(c.m, k)
	}
	c.mu.Unlock()
}

type redisCache struct{ rdb *redis.Client }

func (r *redisCache) Get(ctx context.Context, key string) (string, bool) {
	v, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (r *redisCache) Set(ctx context.Context, key, val string, ttl time.Duration) {
	r.rdb.Set(ctx, key, val, ttl)
}

func (r *redisCache) Del(ctx context.Context, keys ...string) {
	r.rdb.Del(ctx, keys...)
}

func NewCache(ctx context.Context, cfg Config, log *zap.Logger) Cache {
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err == nil {
			rdb := redis.NewClient(opt)
			if err := rdb.Ping(ctx).Err(); err == nil {
				log.Info("cache: redis aktif")
				return &redisCache{rdb}
			}
			rdb.Close()
			log.Warn("redis tak terjangkau, fallback in-memory")
		} else {
			log.Warn("REDIS_URL invalid, fallback in-memory", zapErr(err))
		}
	} else {
		log.Info("REDIS_URL kosong, cache in-memory")
	}
	return &memCache{m: map[string]memEntry{}}
}
