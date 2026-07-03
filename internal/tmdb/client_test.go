package tmdb

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTMDBIDByTVDB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/find/328708" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("external_source"); got != "tvdb_id" {
			t.Fatalf("unexpected external_source: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"movie_results": [],
			"person_results": [],
			"tv_results": [{"id": 67744, "name": "Mindhunter"}],
			"tv_episode_results": [],
			"tv_season_results": []
		}`))
	}))
	defer srv.Close()

	client := NewClient("test-key")
	client.baseURL = srv.URL + "/3"

	tmdb, err := client.TMDBIDByTVDB(t.Context(), 328708)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 67744 {
		t.Fatalf("unexpected tmdb: %v", tmdb)
	}
}

func TestTMDBIDByTVDBNoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tv_results": []}`))
	}))
	defer srv.Close()

	client := NewClient("test-key")
	client.baseURL = srv.URL + "/3"

	tmdb, err := client.TMDBIDByTVDB(t.Context(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb != nil {
		t.Fatalf("expected nil tmdb, got %v", tmdb)
	}
}

func TestTMDBIDByTVDBWithoutAPIKey(t *testing.T) {
	client := NewClient("")
	tmdb, err := client.TMDBIDByTVDB(t.Context(), 328708)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb != nil {
		t.Fatalf("expected nil tmdb without api key, got %v", tmdb)
	}
}
