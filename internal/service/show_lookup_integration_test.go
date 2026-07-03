//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/config"
	"github.com/alireza/tvtime2serializd/internal/mapping"
	"github.com/alireza/tvtime2serializd/internal/platform/postgres"
	redisClient "github.com/alireza/tvtime2serializd/internal/platform/redis"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tmdb"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/alireza/tvtime2serializd/internal/wikidata"
)

func TestEnsureShowResolvesTulsaKingViaTMDBAPI(t *testing.T) {
	cfg := config.Load()
	if cfg.TMDBAPIKey == "" {
		t.Skip("TMDB_API_KEY not set")
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	rdb, err := redisClient.NewClient(cfg.RedisURL)
	if err != nil {
		t.Skip(err)
	}
	defer rdb.Close()

	tvdb := int64(413215)
	_ = rdb.Del(ctx, "tvdb:tmdb:413215").Err()

	wiki := wikidata.NewClient()
	resolver := mapping.NewTMDBResolver(mapping.TMDBResolverDeps{
		Wikidata: wiki,
		TMDB:     tmdb.NewClient(cfg.TMDBAPIKey),
	})
	shows := repository.NewShowCatalog(pool)
	svc := NewShowLookupService(ShowLookupDeps{
		TMDBCache:  cache.NewTMDBCache(rdb),
		Shows:      shows,
		Unresolved: repository.NewUnresolvedShowRepository(pool),
		Resolver:   resolver,
	})

	title := "Tulsa King"
	year := 2022
	showID, tmdbID, err := svc.EnsureShow(ctx, tvtime.ExportShow{
		ID:    tvtime.ExternalIDs{TVDB: &tvdb},
		Title: &title,
		Year:  &year,
	}, tvdb)
	if err != nil {
		t.Fatal(err)
	}
	if tmdbID == nil || *tmdbID != 153312 {
		t.Fatalf("expected tmdb 153312, got %v", tmdbID)
	}

	rec, err := shows.GetByTVDBID(ctx, tvdb)
	if err != nil {
		t.Fatal(err)
	}
	if rec.TMDBID == nil || *rec.TMDBID != 153312 {
		t.Fatalf("postgres tmdb not updated: %v", rec.TMDBID)
	}
	if rec.ID != showID {
		t.Fatalf("unexpected show id: got %d want %d", rec.ID, showID)
	}
}
