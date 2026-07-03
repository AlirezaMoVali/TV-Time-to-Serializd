package mapping

import (
	"context"
	"errors"
	"testing"

	"github.com/alireza/tvtime2serializd/internal/repository"
)

type stubWiki struct {
	tvdb map[int64]*int
	imdb map[string]*int
	err  error
}

func (s *stubWiki) TMDBIDByTVDB(_ context.Context, tvdbID int64) (*int, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tvdb[tvdbID], nil
}

func (s *stubWiki) TMDBIDByIMDB(_ context.Context, imdbID string) (*int, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.imdb[imdbID], nil
}

type stubTMDB struct {
	tvdb map[int64]*int
	err  error
}

func (s *stubTMDB) TMDBIDByTVDB(_ context.Context, tvdbID int64) (*int, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tvdb[tvdbID], nil
}

func TestTMDBResolverFallsBackToIMDB(t *testing.T) {
	id := 67744
	resolver := &TMDBResolver{
		wikidata: &stubWiki{tvdb: map[int64]*int{}, imdb: map[string]*int{"tt5290382": &id}},
		tmdb:     &stubTMDB{tvdb: map[int64]*int{}},
	}

	imdb := "tt5290382"
	tmdb, err := resolver.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: int64Ptr(999),
		IMDBID: &imdb,
		Title:  "MINDHUNTER",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 67744 {
		t.Fatalf("unexpected tmdb: %v", tmdb)
	}
}

func TestTMDBResolverDoesNotSearchByTitle(t *testing.T) {
	id := 67744
	resolver := &TMDBResolver{
		wikidata: &stubWiki{tvdb: map[int64]*int{}, imdb: map[string]*int{}},
		tmdb:     &stubTMDB{tvdb: map[int64]*int{}},
	}

	tmdb, err := resolver.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		Title: "MINDHUNTER",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmdb != nil {
		t.Fatalf("expected nil without tvdb/imdb, got %v", tmdb)
	}
	_ = id
}

func TestTMDBResolverReturnsNilWhenUnresolved(t *testing.T) {
	resolver := &TMDBResolver{
		wikidata: &stubWiki{
			tvdb: map[int64]*int{},
			imdb: map[string]*int{},
			err:  errors.New("wikidata query failed (502)"),
		},
		tmdb: &stubTMDB{tvdb: map[int64]*int{}},
	}

	tvdb := int64Ptr(75886)
	tmdb, err := resolver.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: tvdb,
		Title:  "Luther",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmdb != nil {
		t.Fatalf("unexpected tmdb: %v", tmdb)
	}
}

func TestTMDBResolverFallsBackToTMDBAPI(t *testing.T) {
	id := 81239
	tvdb := int64(328708)
	resolver := &TMDBResolver{
		wikidata: &stubWiki{tvdb: map[int64]*int{}},
		tmdb:     &stubTMDB{tvdb: map[int64]*int{tvdb: &id}},
	}

	tmdb, err := resolver.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "I Am the Night",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 81239 {
		t.Fatalf("unexpected tmdb: %v", tmdb)
	}
}

func TestTMDBResolverFallsBackToTMDBAPIOnWikidataError(t *testing.T) {
	id := 153312
	tvdb := int64(413215)
	resolver := &TMDBResolver{
		wikidata: &stubWiki{
			tvdb: map[int64]*int{},
			err:  errors.New("wikidata query failed (502)"),
		},
		tmdb: &stubTMDB{tvdb: map[int64]*int{tvdb: &id}},
	}

	tmdb, err := resolver.ResolveTMDBID(context.Background(), repository.TMDBLookupInput{
		TVDBID: &tvdb,
		Title:  "Tulsa King",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 153312 {
		t.Fatalf("unexpected tmdb: %v", tmdb)
	}
}

func int64Ptr(v int64) *int64 { return &v }
