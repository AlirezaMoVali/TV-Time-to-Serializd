package postgres

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alireza/tvtime2serializd/internal/safenum"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultMaxConns = 36

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = maxConns()
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

func maxConns() int32 {
	raw := os.Getenv("DATABASE_MAX_CONNS")
	if raw == "" {
		return defaultMaxConns
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultMaxConns
	}
	return safenum.ClampInt32(int32(n), 1, 256)
}
