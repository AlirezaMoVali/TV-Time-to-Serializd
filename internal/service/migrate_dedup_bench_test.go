package service

import (
	"testing"

	"github.com/alireza/tvtime2serializd/internal/tvtime"
)

func BenchmarkMergeTrackedShowIDs(b *testing.B) {
	library := make(map[int]struct{}, 500)
	imported := make(map[int]struct{}, 200)
	for i := range 500 {
		library[i] = struct{}{}
	}
	for i := 200; i < 400; i++ {
		imported[i] = struct{}{}
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = mergeTrackedShowIDs(library, imported)
	}
}

func BenchmarkClassifyResolvedShows(b *testing.B) {
	resolved := make([]resolvedShow, 0, 1000)
	existing := make(map[int]struct{}, 200)
	for i := range 1000 {
		id := 1000 + i
		if i%5 == 0 {
			existing[id] = struct{}{}
		}
		t := id
		resolved = append(resolved, resolvedShow{
			ref:    ShowRef{Name: "Show", TVDBID: int64Ptr(int64(i))},
			tmdbID: &t,
		})
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _, _, _ = classifyResolvedShows(resolved, existing)
	}
}

func BenchmarkFilterPendingImports(b *testing.B) {
	pending := make([]pendingImport, 0, 500)
	tracked := make(map[int]struct{}, 100)
	for i := range 500 {
		pending = append(pending, pendingImport{tmdbID: i, show: tvtime.ExportShow{}})
		if i%5 == 0 {
			tracked[i] = struct{}{}
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = filterPendingImports(pending, tracked)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
