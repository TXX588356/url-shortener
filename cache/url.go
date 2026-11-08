package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type URLCache struct {
	client *redis.Client
}

func NewURLCache(client *redis.Client) *URLCache {
	return &URLCache{client: client}
}

func (c *URLCache) Set(ctx context.Context, shortCode, originalURL string, ttl time.Duration) error {
	key := "url:" + shortCode

	if err := c.client.Set(ctx, key, originalURL, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set Redis cache: %w", err)
	}

	return nil
}

func (c *URLCache) Get(ctx context.Context, shortCode string) (string, error) {
	key := "url:" + shortCode
	originalURL, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get Redis cache: %w", err)
	}

	return originalURL, nil
}

func (c *URLCache) Delete(ctx context.Context, shortCode string) error {
	key := "url:" + shortCode

	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete redis key: %w", err)
	}

	return nil
}