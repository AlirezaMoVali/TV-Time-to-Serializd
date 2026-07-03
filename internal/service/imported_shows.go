package service

import (
	"context"

	"github.com/alireza/tvtime2serializd/internal/account"
	"github.com/alireza/tvtime2serializd/internal/applog"
)

func (s *MigrateService) previouslyImported(ctx context.Context, serializdEmail string) (map[int]struct{}, error) {
	fromRedis, err := s.importedShows.All(ctx, serializdEmail)
	if err != nil {
		return nil, err
	}
	if len(fromRedis) > 0 {
		return fromRedis, nil
	}

	fromDB, err := s.importedShowsDB.All(ctx, account.Hash(serializdEmail))
	if err != nil {
		return nil, err
	}
	return fromDB, nil
}

func (s *MigrateService) recordImported(ctx context.Context, serializdEmail string, tmdbID int) {
	accountHash := account.Hash(serializdEmail)
	applog.LogBestEffort(
		s.importedShowsDB.Add(ctx, accountHash, tmdbID),
		"record imported show in database",
		"tmdb_id", tmdbID,
	)
	applog.LogBestEffort(
		s.importedShows.Add(ctx, serializdEmail, tmdbID),
		"record imported show in cache",
		"tmdb_id", tmdbID,
	)
}
