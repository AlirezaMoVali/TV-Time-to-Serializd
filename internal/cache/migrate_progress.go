package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const migrateProgressTTL = 24 * time.Hour

type MigrateProgressCache struct {
	client *redis.Client
}

func NewMigrateProgressCache(client *redis.Client) *MigrateProgressCache {
	return &MigrateProgressCache{client: client}
}

func (c *MigrateProgressCache) Set(ctx context.Context, jobID uuid.UUID, progress any) error {
	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	key := migrateProgressKey(jobID)
	if err := c.client.Set(ctx, key, data, migrateProgressTTL).Err(); err != nil {
		return fmt.Errorf("cache migrate progress: %w", err)
	}
	return nil
}

func (c *MigrateProgressCache) Get(ctx context.Context, jobID uuid.UUID, dest any) (bool, error) {
	data, err := c.client.Get(ctx, migrateProgressKey(jobID)).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

func migrateProgressKey(jobID uuid.UUID) string {
	return fmt.Sprintf("migrate:job:%s", jobID)
}
