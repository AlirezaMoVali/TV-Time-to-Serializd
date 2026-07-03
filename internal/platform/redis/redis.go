package redis

import (
	"context"
	"fmt"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/redis/go-redis/v9"
)

func NewClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		applog.LogBestEffort(client.Close(), "close redis after ping failure")
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
