package tvtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestFetchObjectsPaginatesAllPages(t *testing.T) {
	const pageLimit = 100
	const total = 423

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("page_offset"))
		count := pageLimit
		if offset+count > total {
			count = total - offset
		}

		objects := make([]map[string]any, count)
		for i := 0; i < count; i++ {
			idx := offset + i
			objects[i] = map[string]any{
				"uuid": fmt.Sprintf("watch-%d", idx),
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"objects": objects,
			},
		})
	}))
	defer srv.Close()

	client := NewClient()
	client.sidecarBase = srv.URL
	got, err := client.fetchObjects("token", "https://msapi.tvtime.com/watches", "episode", pageLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != total {
		t.Fatalf("expected %d objects, got %d", total, len(got))
	}
}

func TestFetchObjectsRetriesTransientPageFailures(t *testing.T) {
	const pageLimit = 100
	attempts := map[int]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("page_offset"))
		attempts[offset]++

		if offset == 100 && attempts[offset] == 1 {
			http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
			return
		}

		count := pageLimit
		if offset == 200 {
			count = 23
		}
		objects := make([]map[string]any, count)
		for i := 0; i < count; i++ {
			objects[i] = map[string]any{"uuid": fmt.Sprintf("%d-%d", offset, i)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"objects": objects},
		})
	}))
	defer srv.Close()

	client := NewClient()
	client.sidecarBase = srv.URL
	got, err := client.fetchObjects("token", "https://msapi.tvtime.com/watches", "episode", pageLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 223 {
		t.Fatalf("expected 223 objects, got %d", len(got))
	}
	if attempts[100] < 2 {
		t.Fatalf("expected retry on failed page, got %d attempts", attempts[100])
	}
}

func TestBuildWatchedAtMapMatchesFollowedShows(t *testing.T) {
	followed := map[string]struct{}{
		"show-1": {},
	}
	watches := []map[string]any{
		{
			"series_uuid": "show-1",
			"episode_id":  float64(12345),
			"watched_at":  "2024-01-21T00:33:46.403717Z",
			"rewatch_count": float64(1),
		},
		{
			"series_uuid": "show-2",
			"episode_id":  float64(99999),
		},
		{
			"series_uuid": "show-1",
			"episode_id":  "67890",
		},
	}

	got := buildWatchedAtMap(watches, followed)
	if _, ok := got["12345"]; !ok {
		t.Fatalf("expected watched episode 12345, got %#v", got)
	}
	if _, ok := got["67890"]; !ok {
		t.Fatalf("expected watched episode 67890, got %#v", got)
	}
	if _, ok := got["99999"]; ok {
		t.Fatalf("did not expect unfollowed show watch to be indexed")
	}
}

func TestWatchMapKeyNormalizesNumericIDs(t *testing.T) {
	if got := watchMapKey(float64(12345)); got != "12345" {
		t.Fatalf("unexpected key: %q", got)
	}
	if got := watchMapKey(json.Number("67890")); got != "67890" {
		t.Fatalf("unexpected key: %q", got)
	}
	if got := watchMapKey(" 42 "); got != "42" {
		t.Fatalf("unexpected key: %q", got)
	}
}

func TestCountWatchedEpisodesSkipsSpecials(t *testing.T) {
	id := int64(1)
	shows := []ExportShow{{
		Seasons: []ExportSeason{{
			Number: 1,
			Episodes: []ExportEpisode{
				{IsWatched: true, Special: false},
				{IsWatched: true, Special: true},
			},
		}, {
			Number: 0,
			Episodes: []ExportEpisode{
				{IsWatched: true, Special: true, ID: ExternalIDs{TVDB: &id}},
			},
		}},
	}}

	if got := countWatchedEpisodes(shows); got != 1 {
		t.Fatalf("expected 1 watched episode, got %d", got)
	}
}

func TestValidateTokens(t *testing.T) {
	if err := ValidateTokens(nil); err == nil {
		t.Fatal("expected error for nil tokens")
	}
	if err := ValidateTokens(&Tokens{JWTToken: "x", UserID: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractJWTPrefersNestedData(t *testing.T) {
	got := extractJWT(map[string]any{
		"data": map[string]any{"jwt_token": "nested"},
	})
	if got != "nested" {
		t.Fatalf("unexpected jwt: %q", got)
	}
}

func TestFetchObjectsStopsOnDuplicateFirstUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		objects := []map[string]any{{"uuid": "same-uuid"}}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"objects": objects},
		})
	}))
	defer srv.Close()

	client := NewClient()
	client.sidecarBase = srv.URL
	got, err := client.fetchObjects("token", "https://msapi.tvtime.com/watches", "episode", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected duplicate guard to stop after first page, got %d", len(got))
	}
	if !strings.HasPrefix(got[0]["uuid"].(string), "same-uuid") {
		t.Fatalf("unexpected object: %#v", got[0])
	}
}
