package cache

import (
	"context"
	"testing"

	"github.com/alireza/tvtime2serializd/internal/config"
	"github.com/alireza/tvtime2serializd/internal/platform/redis"
	"github.com/alireza/tvtime2serializd/internal/repository"
)

func TestTMDBCacheWarm(t *testing.T) {
	cfg := config.Load()
	rdb, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		t.Skip(err)
	}
	defer rdb.Close()

	ctx := context.Background()
	tvdbResolved := int64(900001)
	tvdbNegative := int64(900002)
	_ = rdb.Del(ctx, tmdbKey(tvdbResolved), tmdbKey(tvdbNegative)).Err()

	cache := NewTMDBCache(rdb)
	resolved, negative, err := cache.Warm(ctx, []repository.TVDBTMDBMapping{
		{TVDBID: tvdbResolved, TMDBID: 12345},
	}, []int64{tvdbNegative})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != 1 || negative != 1 {
		t.Fatalf("expected 1 resolved and 1 negative, got %d and %d", resolved, negative)
	}

	got, found, err := cache.Get(ctx, tvdbResolved)
	if err != nil || !found || got == nil || *got != 12345 {
		t.Fatalf("resolved cache: got=%v found=%v err=%v", got, found, err)
	}

	got, found, err = cache.Get(ctx, tvdbNegative)
	if err != nil || !found || got != nil {
		t.Fatalf("negative cache: got=%v found=%v err=%v", got, found, err)
	}

	_ = rdb.Del(ctx, tmdbKey(tvdbResolved), tmdbKey(tvdbNegative)).Err()
}
