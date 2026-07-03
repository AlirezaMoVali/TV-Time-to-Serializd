package tvtime

import "testing"

func BenchmarkBuildWatchedAtMap(b *testing.B) {
	followed := make(map[string]struct{}, 500)
	watches := make([]map[string]any, 0, 5000)
	for i := range 500 {
		uuid := "series-" + string(rune('a'+i%26))
		followed[uuid] = struct{}{}
		for j := range 10 {
			watches = append(watches, map[string]any{
				"series_uuid":  uuid,
				"episode_id":   float64(i*100 + j),
				"watched_at":   "2024-01-01T12:00:00Z",
				"rewatch_count": float64(j % 3),
			})
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = buildWatchedAtMap(watches, followed)
	}
}
