package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/alireza/tvtime2serializd/internal/account"
	"github.com/redis/go-redis/v9"
)

// ImportedShowsCache remembers TMDB show IDs already imported per Serializd account.
// Redis keys use account.Hash(email) — plaintext email is never stored.
type ImportedShowsCache struct {
	client *redis.Client
}

func NewImportedShowsCache(client *redis.Client) *ImportedShowsCache {
	return &ImportedShowsCache{client: client}
}

func (c *ImportedShowsCache) Add(ctx context.Context, email string, tmdbID int) error {
	if tmdbID <= 0 {
		return nil
	}
	if err := c.client.SAdd(ctx, importedShowsKey(email), tmdbID).Err(); err != nil {
		return fmt.Errorf("record imported show: %w", err)
	}
	return nil
}

func (c *ImportedShowsCache) All(ctx context.Context, email string) (map[int]struct{}, error) {
	members, err := c.client.SMembers(ctx, importedShowsKey(email)).Result()
	if err != nil {
		return nil, fmt.Errorf("list imported shows: %w", err)
	}
	out := make(map[int]struct{}, len(members))
	for _, member := range members {
		id, err := strconv.Atoi(member)
		if err != nil || id <= 0 {
			continue
		}
		out[id] = struct{}{}
	}
	return out, nil
}

func importedShowsKey(email string) string {
	return "serializd:imported:" + account.Hash(email)
}
