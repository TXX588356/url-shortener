package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	err := client.Ping(context.Background()).Err()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping to redis: %w", err)
	}

	return client, nil
}
