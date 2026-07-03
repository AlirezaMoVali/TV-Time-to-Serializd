package service

import (
	"testing"

	"github.com/alireza/tvtime2serializd/internal/tvtime"
)

func TestClassifyResolvedShows_DedupesPendingByTMDBID(t *testing.T) {
	tmdb := 1408
	resolved := []resolvedShow{
		{ref: ShowRef{Name: "House A"}, tmdbID: &tmdb},
		{ref: ShowRef{Name: "House B"}, tmdbID: &tmdb},
	}

	pending, skipped, notFound, skippedRefs := classifyResolvedShows(resolved, nil)
	if len(notFound) != 0 {
		t.Fatalf("notFound = %v", notFound)
	}
	if len(pending) != 1 || pending[0].tmdbID != tmdb {
		t.Fatalf("pending = %+v, want one item", pending)
	}
	if skipped != 1 || len(skippedRefs) != 1 {
		t.Fatalf("skipped = %d refs=%v, want 1 duplicate skip", skipped, skippedRefs)
	}
}

func TestFilterPendingImports(t *testing.T) {
	pending := []pendingImport{
		{tmdbID: 1, show: tvtime.ExportShow{}},
		{tmdbID: 2, show: tvtime.ExportShow{}},
		{tmdbID: 3, show: tvtime.ExportShow{}},
	}
	tracked := map[int]struct{}{2: {}}

	filtered, skipped := filterPendingImports(pending, tracked)
	if skipped != 1 || len(filtered) != 2 {
		t.Fatalf("filtered=%+v skipped=%d", filtered, skipped)
	}
	if filtered[0].tmdbID != 1 || filtered[1].tmdbID != 3 {
		t.Fatalf("unexpected filtered order: %+v", filtered)
	}
}

func TestMergeTrackedShowIDs(t *testing.T) {
	got := mergeTrackedShowIDs(map[int]struct{}{1: {}, 2: {}}, map[int]struct{}{2: {}, 3: {}})
	if len(got) != 3 {
		t.Fatalf("merged = %v, want 3 ids", got)
	}
}
