package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xltxb/PetManage/internal/config"
)

// RedisClient wraps go-redis with cache hit rate tracking.
type RedisClient struct {
	rdb    *redis.Client
	hits   atomic.Int64
	misses atomic.Int64
}

// NewRedisClient creates a Redis client and verifies connectivity.
// Returns the client even if Redis is unavailable — the caller can check
// Available() and skip caching when Redis is down.
func NewRedisClient(cfg config.RedisConfig) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	rc := &RedisClient{rdb: rdb}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return rc
	}

	return rc
}

// Available returns true when Redis is connected.
func (rc *RedisClient) Available() bool {
	if rc.rdb == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return rc.rdb.Ping(ctx).Err() == nil
}

// Close shuts down the Redis connection pool.
func (rc *RedisClient) Close() error {
	if rc.rdb == nil {
		return nil
	}
	return rc.rdb.Close()
}

// HitRate returns the cache hit ratio as a float between 0 and 1.
func (rc *RedisClient) HitRate() float64 {
	hits := rc.hits.Load()
	misses := rc.misses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// GetJSON retrieves a cached JSON value and unmarshals it into dest.
// Returns true on cache hit, false on miss.
func (rc *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) bool {
	if !rc.Available() {
		rc.misses.Add(1)
		return false
	}

	data, err := rc.rdb.Get(ctx, key).Bytes()
	if err != nil {
		rc.misses.Add(1)
		return false
	}

	if err := json.Unmarshal(data, dest); err != nil {
		rc.misses.Add(1)
		return false
	}

	rc.hits.Add(1)
	return true
}

// SetJSON marshals value to JSON and stores it in Redis with the given TTL.
func (rc *RedisClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !rc.Available() {
		return fmt.Errorf("redis unavailable")
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return rc.rdb.Set(ctx, key, data, ttl).Err()
}

// Delete removes one or more keys from Redis. Missing keys are silently ignored.
func (rc *RedisClient) Delete(ctx context.Context, keys ...string) error {
	if !rc.Available() {
		return nil
	}
	return rc.rdb.Del(ctx, keys...).Err()
}

// InvalidatePattern deletes all keys matching a glob pattern (e.g. "cache:*").
func (rc *RedisClient) InvalidatePattern(ctx context.Context, pattern string) error {
	if !rc.Available() {
		return nil
	}

	iter := rc.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := rc.rdb.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// Client returns the underlying go-redis client for direct use.
func (rc *RedisClient) Client() *redis.Client {
	return rc.rdb
}
