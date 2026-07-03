package server

import (
	"context"
	"fmt"

	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// WarmTMDBCache preloads Redis TVDB→TMDB mappings from Postgres on startup.
func WarmTMDBCache(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client) (resolved, negative int, err error) {
	shows := repository.NewShowCatalog(pool)
	mappings, err := shows.ListTVDBTMDBMappings(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("load tvdb tmdb mappings: %w", err)
	}

	unresolved := repository.NewUnresolvedShowRepository(pool)
	unresolvedIDs, err := unresolved.ListUnresolvedTVDBIDs(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("load unresolved tvdb ids: %w", err)
	}

	return cache.NewTMDBCache(rdb).Warm(ctx, mappings, unresolvedIDs)
}
