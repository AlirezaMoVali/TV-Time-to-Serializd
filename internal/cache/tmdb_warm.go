package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/alireza/tvtime2serializd/internal/repository"
)

const tmdbWarmBatchSize = 500

// Warm loads TVDB→TMDB mappings and negative-cache entries into Redis.
func (c *TMDBCache) Warm(ctx context.Context, resolved []repository.TVDBTMDBMapping, unresolvedTVDBIDs []int64) (resolvedCount, negativeCount int, err error) {
	resolvedCount, err = c.warmResolved(ctx, resolved)
	if err != nil {
		return resolvedCount, 0, err
	}
	negativeCount, err = c.warmNegative(ctx, unresolvedTVDBIDs)
	if err != nil {
		return resolvedCount, negativeCount, err
	}
	return resolvedCount, negativeCount, nil
}

func (c *TMDBCache) warmResolved(ctx context.Context, resolved []repository.TVDBTMDBMapping) (int, error) {
	written := 0
	for i := 0; i < len(resolved); i += tmdbWarmBatchSize {
		end := min(i+tmdbWarmBatchSize, len(resolved))
		pipe := c.client.Pipeline()
		for _, m := range resolved[i:end] {
			pipe.Set(ctx, tmdbKey(m.TVDBID), strconv.Itoa(m.TMDBID), tmdbMappingTTL)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return written, fmt.Errorf("warm resolved tmdb cache: %w", err)
		}
		written += end - i
	}
	return written, nil
}

func (c *TMDBCache) warmNegative(ctx context.Context, unresolvedTVDBIDs []int64) (int, error) {
	written := 0
	for i := 0; i < len(unresolvedTVDBIDs); i += tmdbWarmBatchSize {
		end := min(i+tmdbWarmBatchSize, len(unresolvedTVDBIDs))
		pipe := c.client.Pipeline()
		for _, tvdbID := range unresolvedTVDBIDs[i:end] {
			pipe.Set(ctx, tmdbKey(tvdbID), "0", tmdbMappingTTL)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return written, fmt.Errorf("warm negative tmdb cache: %w", err)
		}
		written += end - i
	}
	return written, nil
}
