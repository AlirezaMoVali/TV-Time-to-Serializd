package cache

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type SerializdSessionCache struct {
	client *redis.Client
}

func NewSerializdSessionCache(client *redis.Client) *SerializdSessionCache {
	return &SerializdSessionCache{client: client}
}

func (c *SerializdSessionCache) Set(ctx context.Context, tokenID uuid.UUID, jwtToken string) error {
	key := serializdSessionKey(tokenID)
	if err := c.client.Set(ctx, key, jwtToken, sessionTTL).Err(); err != nil {
		return fmt.Errorf("cache serializd session: %w", err)
	}
	return nil
}

func (c *SerializdSessionCache) Get(ctx context.Context, tokenID uuid.UUID) (string, error) {
	return c.client.Get(ctx, serializdSessionKey(tokenID)).Result()
}

func serializdSessionKey(tokenID uuid.UUID) string {
	return fmt.Sprintf("serializd:token:%s", tokenID)
}
