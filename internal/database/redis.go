package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewCacheService(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func InitRedis(ctx context.Context, cfg *configs.Config) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "redis:6379", // We change the Address to redis:6379 when connecting via Docker instead of cfg.Redis.Addr
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		Username: "default",
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("failed to connect to Redis: %v", err)
		return nil, err
	}

	log.Println("⚡️ Successfully connected to Redis Cache!")
	return &RedisCache{client: rdb}, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	marshaledValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for Redis: %w", err)
	}
	return r.client.Set(ctx, key, marshaledValue, expiration).Err()
}

func (r *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("key '%s' not found in Redis", key)
	} else if err != nil {
		return fmt.Errorf("failed to get value from Redis: %w", err)
	}
	return json.Unmarshal([]byte(val), dest)
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) RawClient() *redis.Client {
	return r.client
}

var _ service.CacheService = (*RedisCache)(nil)
