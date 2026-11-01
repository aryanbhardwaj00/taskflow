package infra

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/blockaryan/taskflow/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(cfg config.RedisConfig) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           0,               // Default DB 0
		PoolSize:     20,              // Connection pool size
		MinIdleConns: 5,               // Maintain minimum idle connections
		DialTimeout:  5 * time.Second, // Timeout for initial connection
		ReadTimeout:  3 * time.Second, // Timeout for socket reads
		WriteTimeout: 3 * time.Second, // Timeout for socket writes
	})
	// Verify connectivity with immediate ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("failed to ping redis at %s:%s: %w", cfg.Host, cfg.Port, err)
	}

	log.Printf("Connected to Redis at %s:%s successfully", cfg.Host, cfg.Port)

	return &RedisClient{Client: rdb}, nil
}

// Set stores a key-value pair with an expiration TTL.
func (r *RedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.Client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a string value by key. Returns redis.Nil if key does not exist.
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, key).Result()
}

// Del removes one or more keys.
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.Client.Del(ctx, keys...).Err()
}

// Close gracefully terminates the Redis connection pool.
func (r *RedisClient) Close() {
	if r != nil && r.Client != nil {
		if err := r.Client.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v\n", err)
		} else {
			log.Println("Redis connection pool closed.")
		}
	}
}
