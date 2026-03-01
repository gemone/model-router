// Package cache provides multi-level caching support with Redis/DragonflyDB L2 cache.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// L2CacheConfig configures the L2 cache behavior
type L2CacheConfig struct {
	// Redis connection settings
	Addr     string // Redis server address (default: localhost:6379)
	Password string // Redis password (default: "")
	DB       int    // Redis database number (default: 0)

	// Cache behavior
	DefaultTTL   time.Duration // Default TTL for cache entries (default: 1 hour)
	KeyPrefix    string         // Prefix for all cache keys (default: "model-router:")
	MaxRetries   int            // Maximum number of retries (default: 3)
	PoolSize     int            // Connection pool size (default: 10)
}

// DefaultL2CacheConfig returns default L2 cache configuration
func DefaultL2CacheConfig() *L2CacheConfig {
	return &L2CacheConfig{
		Addr:       "localhost:6379",
		Password:   "",
		DB:         0,
		DefaultTTL: time.Hour,
		KeyPrefix:  "model-router:",
		MaxRetries: 3,
		PoolSize:   10,
	}
}

// L2Cache provides Redis/DragonflyDB caching for model responses and embeddings
type L2Cache struct {
	client *redis.Client
	config *L2CacheConfig
}

// NewL2Cache creates a new L2 cache instance with default configuration
func NewL2Cache() (*L2Cache, error) {
	return NewL2CacheWithConfig(DefaultL2CacheConfig())
}

// NewL2CacheWithConfig creates a new L2 cache instance with custom configuration
func NewL2CacheWithConfig(config *L2CacheConfig) (*L2Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		MaxRetries:   config.MaxRetries,
		PoolSize:     config.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &L2Cache{
		client: client,
		config: config,
	}, nil
}

// Get retrieves a value from the cache
func (c *L2Cache) Get(ctx context.Context, key string, dest interface{}) error {
	fullKey := c.config.KeyPrefix + key

	val, err := c.client.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss
			return fmt.Errorf("cache miss")
		}
		return fmt.Errorf("failed to get from cache: %w", err)
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cached value: %w", err)
	}

	return nil
}

// Set stores a value in the cache with the default TTL
func (c *L2Cache) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithTTL(ctx, key, value, c.config.DefaultTTL)
}

// SetWithTTL stores a value in the cache with a specific TTL
func (c *L2Cache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := c.config.KeyPrefix + key

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := c.client.Set(ctx, fullKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	return nil
}

// Delete removes a value from the cache
func (c *L2Cache) Delete(ctx context.Context, key string) error {
	fullKey := c.config.KeyPrefix + key

	if err := c.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	return nil
}

// DeletePattern removes all values matching a pattern from the cache
func (c *L2Cache) DeletePattern(ctx context.Context, pattern string) error {
	fullPattern := c.config.KeyPrefix + pattern

	iter := c.client.Scan(ctx, 0, fullPattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("failed to delete key %s: %w", iter.Val(), err)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	return nil
}

// Clear removes all entries with the configured key prefix from the cache
func (c *L2Cache) Clear(ctx context.Context) error {
	pattern := c.config.KeyPrefix + "*"
	return c.DeletePattern(ctx, pattern)
}

// Exists checks if a key exists in the cache
func (c *L2Cache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := c.config.KeyPrefix + key

	count, err := c.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return count > 0, nil
}

// GetTTL returns the remaining time to live for a key
func (c *L2Cache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := c.config.KeyPrefix + key

	ttl, err := c.client.TTL(ctx, fullKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}

	return ttl, nil
}

// SetTTL updates the TTL for an existing key
func (c *L2Cache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := c.config.KeyPrefix + key

	if err := c.client.Expire(ctx, fullKey, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set TTL: %w", err)
	}

	return nil
}

// Increment atomically increments a numeric value in the cache
func (c *L2Cache) Increment(ctx context.Context, key string) (int64, error) {
	fullKey := c.config.KeyPrefix + key

	val, err := c.client.Incr(ctx, fullKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment: %w", err)
	}

	return val, nil
}

// GetMulti retrieves multiple values from the cache
func (c *L2Cache) GetMulti(ctx context.Context, keys []string, dest map[string]interface{}) error {
	if len(keys) == 0 {
		return nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = c.config.KeyPrefix + key
	}

	vals, err := c.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return fmt.Errorf("failed to get multiple values: %w", err)
	}

	for i, val := range vals {
		if val == nil {
			continue // Cache miss
		}

		if strVal, ok := val.(string); ok {
			if dest[keys[i]] != nil {
				if err := json.Unmarshal([]byte(strVal), dest[keys[i]]); err != nil {
					return fmt.Errorf("failed to unmarshal key %s: %w", keys[i], err)
				}
			}
		}
	}

	return nil
}

// SetMulti stores multiple values in the cache
func (c *L2Cache) SetMulti(ctx context.Context, items map[string]interface{}) error {
	if len(items) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()

	for key, value := range items {
		fullKey := c.config.KeyPrefix + key

		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal key %s: %w", key, err)
		}

		pipe.Set(ctx, fullKey, data, c.config.DefaultTTL)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set multiple values: %w", err)
	}

	return nil
}

// GetStats returns cache statistics
func (c *L2Cache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := c.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}

	dbSize, err := c.client.DBSize(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB size: %w", err)
	}

	return map[string]interface{}{
		"info":     info,
		"db_size":  dbSize,
		"key_prefix": c.config.KeyPrefix,
	}, nil
}

// Close closes the cache connection
func (c *L2Cache) Close() error {
	return c.client.Close()
}

// Ping checks if the cache connection is alive
func (c *L2Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Cache helpers for specific use cases

// CacheResponseKey generates a cache key for model responses
func CacheResponseKey(model string, requestHash string) string {
	return fmt.Sprintf("response:%s:%s", model, requestHash)
}

// CacheEmbeddingKey generates a cache key for embeddings
func CacheEmbeddingKey(model string, text string) string {
	return fmt.Sprintf("embedding:%s:%s", model, text)
}

// CacheHealthKey generates a cache key for health check results
func CacheHealthKey(providerID string) string {
	return fmt.Sprintf("health:%s", providerID)
}
