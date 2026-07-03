package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const sessionTTL = 24 * time.Hour

type SessionCache struct {
	client *redis.Client
}

func NewSessionCache(client *redis.Client) *SessionCache {
	return &SessionCache{client: client}
}

func (c *SessionCache) Set(ctx context.Context, tokenID uuid.UUID, tokens *tvtime.Tokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := sessionKey(tokenID)
	if err := c.client.Set(ctx, key, data, sessionTTL).Err(); err != nil {
		return fmt.Errorf("cache session: %w", err)
	}
	return nil
}

func (c *SessionCache) Get(ctx context.Context, tokenID uuid.UUID) (*tvtime.Tokens, error) {
	data, err := c.client.Get(ctx, sessionKey(tokenID)).Bytes()
	if err != nil {
		return nil, err
	}

	var tokens tvtime.Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &tokens, nil
}

func sessionKey(tokenID uuid.UUID) string {
	return fmt.Sprintf("tvtime:token:%s", tokenID)
}
