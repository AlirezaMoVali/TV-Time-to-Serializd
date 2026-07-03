package service

import (
	"context"
	"testing"

	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/jackc/pgx/v5"
)

type stubTMDBCache struct {
	values map[int64]*int
	found  map[int64]bool
	set    map[int64]*int
}

func (s *stubTMDBCache) Get(_ context.Context, tvdbID int64) (*int, bool, error) {
	if s.found[tvdbID] {
		return s.values[tvdbID], true, nil
	}
	return nil, false, nil
}

func (s *stubTMDBCache) Set(_ context.Context, tvdbID int64, tmdbID *int) error {
	if s.found == nil {
		s.found = make(map[int64]bool)
	}
	if s.set == nil {
		s.set = make(map[int64]*int)
	}
	s.found[tvdbID] = true
	s.set[tvdbID] = tmdbID
	return nil
}

type stubShowCatalog struct {
	byTVDB map[int64]*repository.ShowRecord
	upsert []tvtime.ExportShow
	nextID int64
}

func (s *stubShowCatalog) UpsertWithoutResolver(_ context.Context, show tvtime.ExportShow, _ int64) (int64, error) {
	s.upsert = append(s.upsert, show)
	if show.ID.TVDB != nil {
		if rec, ok := s.byTVDB[*show.ID.TVDB]; ok {
			return rec.ID, nil
		}
		s.nextID++
		rec := &repository.ShowRecord{ID: s.nextID, TVDBID: show.ID.TVDB, Title: deref(show.Title)}
		s.byTVDB[*show.ID.TVDB] = rec
		return rec.ID, nil
	}
	s.nextID++
	return s.nextID, nil
}

func (s *stubShowCatalog) GetByTVDBID(_ context.Context, tvdbID int64) (*repository.ShowRecord, error) {
	rec, ok := s.byTVDB[tvdbID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	copy := *rec
	return &copy, nil
}

func (s *stubShowCatalog) SetTMDBID(_ context.Context, showID int64, tmdbID int) error {
	for _, rec := range s.byTVDB {
		if rec.ID == showID {
			rec.TMDBID = &tmdbID
		}
	}
	return nil
}

type stubUnresolved struct {
	records []repository.TMDBLookupInput
}

func (s *stubUnresolved) Record(_ context.Context, tvdbID *int64, imdbID *string, title string, year *int) error {
	s.records = append(s.records, repository.TMDBLookupInput{
		TVDBID: tvdbID,
		IMDBID: imdbID,
		Title:  title,
		Year:   year,
	})
	return nil
}

type stubResolver struct {
	ids    map[int64]*int
	calls  int
	panic  bool
}

func (s *stubResolver) ResolveTMDBID(_ context.Context, input repository.TMDBLookupInput) (*int, error) {
	s.calls++
	if s.panic {
		panic("resolver should not be called")
	}
	if input.TVDBID == nil {
		return nil, nil
	}
	return s.ids[*input.TVDBID], nil
}

func TestShowLookupServiceUsesRedisCache(t *testing.T) {
	tmdb := 67744
	tvdb := int64(328708)
	cache := &stubTMDBCache{
		values: map[int64]*int{tvdb: &tmdb},
		found:  map[int64]bool{tvdb: true},
		set:    map[int64]*int{},
	}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{
		tvdb: {ID: 1, TVDBID: &tvdb, TMDBID: &tmdb},
	}}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: &stubUnresolved{},
		resolver:   &stubResolver{panic: true},
	}

	got, err := svc.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "Mindhunter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tmdb {
		t.Fatalf("unexpected tmdb: %v", got)
	}
	if len(shows.upsert) != 0 {
		t.Fatalf("expected no upsert on cache hit, got %d", len(shows.upsert))
	}
}

func TestShowLookupServiceUsesPostgresTMDB(t *testing.T) {
	tmdb := 67744
	tvdb := int64(328708)
	cache := &stubTMDBCache{set: map[int64]*int{}}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{
		tvdb: {ID: 1, TVDBID: &tvdb, TMDBID: &tmdb, Title: "Mindhunter"},
	}}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: &stubUnresolved{},
		resolver:   &stubResolver{panic: true},
	}

	got, err := svc.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "Mindhunter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tmdb {
		t.Fatalf("unexpected tmdb: %v", got)
	}
	if cache.set[tvdb] == nil || *cache.set[tvdb] != tmdb {
		t.Fatalf("expected redis cache to be set, got %v", cache.set[tvdb])
	}
}

func TestShowLookupServiceResolveDoesNotUpsert(t *testing.T) {
	tmdb := 81239
	tvdb := int64(328708)
	cache := &stubTMDBCache{set: map[int64]*int{}}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{}}
	resolver := &stubResolver{ids: map[int64]*int{tvdb: &tmdb}}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: &stubUnresolved{},
		resolver:   resolver,
	}

	got, err := svc.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "I Am the Night",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tmdb {
		t.Fatalf("unexpected tmdb: %v", got)
	}
	if len(shows.upsert) != 0 {
		t.Fatalf("ResolveTMDBID should not upsert shows, got %d", len(shows.upsert))
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
}

func TestShowLookupServiceEnsureShowCreatesAndResolves(t *testing.T) {
	tmdb := 81239
	tvdb := int64(328708)
	cache := &stubTMDBCache{set: map[int64]*int{}}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{}}
	unresolved := &stubUnresolved{}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: unresolved,
		resolver:   &stubResolver{ids: map[int64]*int{tvdb: &tmdb}},
	}

	_, got, err := svc.EnsureShow(context.Background(), tvtime.ExportShow{
		ID:    tvtime.ExternalIDs{TVDB: &tvdb},
		Title: strPtr("I Am the Night"),
	}, tvdb)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tmdb {
		t.Fatalf("unexpected tmdb: %v", got)
	}
	if len(shows.upsert) != 1 {
		t.Fatalf("expected show upsert, got %d", len(shows.upsert))
	}
	if cache.set[tvdb] == nil || *cache.set[tvdb] != tmdb {
		t.Fatalf("expected redis cache to be set, got %v", cache.set[tvdb])
	}
}

func TestShowLookupServiceRecordsUnresolved(t *testing.T) {
	tvdb := int64(328708)
	cache := &stubTMDBCache{set: map[int64]*int{}}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{}}
	unresolved := &stubUnresolved{}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: unresolved,
		resolver:   &stubResolver{ids: map[int64]*int{}},
	}

	got, err := svc.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "Unknown Show",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil tmdb, got %v", got)
	}
	if len(unresolved.records) != 1 {
		t.Fatalf("expected unresolved record, got %d", len(unresolved.records))
	}
}

func TestShowLookupServiceRetriesAfterNegativeRedisCache(t *testing.T) {
	tmdb := 153312
	tvdb := int64(413215)
	cache := &stubTMDBCache{
		values: map[int64]*int{tvdb: nil},
		found:  map[int64]bool{tvdb: true},
		set:    map[int64]*int{},
	}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{
		tvdb: {ID: 163, TVDBID: &tvdb, Title: "Tulsa King"},
	}}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: &stubUnresolved{},
		resolver:   &stubResolver{ids: map[int64]*int{tvdb: &tmdb}},
	}

	got, err := svc.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "Tulsa King",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tmdb {
		t.Fatalf("unexpected tmdb: %v", got)
	}
}

func TestShowLookupServiceEnsureShowResolvesExistingWithoutTMDB(t *testing.T) {
	tmdb := 81239
	tvdb := int64(328708)
	cache := &stubTMDBCache{set: map[int64]*int{}}
	shows := &stubShowCatalog{byTVDB: map[int64]*repository.ShowRecord{
		tvdb: {ID: 7, TVDBID: &tvdb, Title: "I Am the Night"},
	}}

	svc := &ShowLookupService{
		cache:      cache,
		shows:      shows,
		unresolved: &stubUnresolved{},
		resolver:   &stubResolver{ids: map[int64]*int{tvdb: &tmdb}},
	}

	_, got, err := svc.EnsureShow(context.Background(), tvtime.ExportShow{
		ID:    tvtime.ExternalIDs{TVDB: &tvdb},
		Title: strPtr("I Am the Night"),
	}, tvdb)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tmdb {
		t.Fatalf("unexpected tmdb: %v", got)
	}
	if len(shows.upsert) != 0 {
		t.Fatalf("expected no new upsert, got %d", len(shows.upsert))
	}
	if shows.byTVDB[tvdb].TMDBID == nil || *shows.byTVDB[tvdb].TMDBID != tmdb {
		t.Fatalf("expected tmdb to be set on show record")
	}
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
