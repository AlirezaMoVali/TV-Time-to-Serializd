package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const tmdbMappingTTL = 30 * 24 * time.Hour

type TMDBCache struct {
	client *redis.Client
}

func NewTMDBCache(client *redis.Client) *TMDBCache {
	return &TMDBCache{client: client}
}

func (c *TMDBCache) Get(ctx context.Context, tvdbID int64) (*int, bool, error) {
	key := tmdbKey(tvdbID)
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get tmdb cache: %w", err)
	}
	if val == "0" {
		return nil, true, nil
	}
	id, err := strconv.Atoi(val)
	if err != nil {
		return nil, false, err
	}
	return &id, true, nil
}

func (c *TMDBCache) Set(ctx context.Context, tvdbID int64, tmdbID *int) error {
	val := "0"
	if tmdbID != nil {
		val = strconv.Itoa(*tmdbID)
	}
	if err := c.client.Set(ctx, tmdbKey(tvdbID), val, tmdbMappingTTL).Err(); err != nil {
		return fmt.Errorf("set tmdb cache: %w", err)
	}
	return nil
}

func tmdbKey(tvdbID int64) string {
	return fmt.Sprintf("tvdb:tmdb:%d", tvdbID)
}
