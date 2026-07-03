package wikidata

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTMDBIDByTVDB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sparql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")
		if query == "" || !strings.Contains(query, `wdt:P4835 "328708"`) {
			t.Fatalf("unexpected query: %s", query)
		}
		w.Header().Set("Content-Type", "application/sparql-results+json")
		_, _ = w.Write([]byte(`{
			"results": {
				"bindings": [{
					"show": {"type": "uri", "value": "http://www.wikidata.org/entity/Q1870246"},
					"showLabel": {"type": "literal", "value": "Mindhunter"},
					"tmdb": {"type": "literal", "value": "67744"}
				}]
			}
		}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.endpoint = srv.URL + "/sparql"

	tmdb, err := client.TMDBIDByTVDB(t.Context(), 328708)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 67744 {
		t.Fatalf("unexpected tmdb id: %v", tmdb)
	}

	// Cached on second call.
	tmdb2, err := client.TMDBIDByTVDB(t.Context(), 328708)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb2 == nil || *tmdb2 != 67744 {
		t.Fatalf("unexpected cached tmdb id: %v", tmdb2)
	}
}

func TestTMDBIDByTVDBNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":{"bindings":[]}}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.endpoint = srv.URL + "/sparql"

	tmdb, err := client.TMDBIDByTVDB(t.Context(), 999999999)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb != nil {
		t.Fatalf("expected nil, got %d", *tmdb)
	}
}

func TestQueryRetriesOn502(t *testing.T) {
	old := wikidataRetryDelays
	wikidataRetryDelays = []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}
	t.Cleanup(func() { wikidataRetryDelays = old })

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"results":{"bindings":[{"tmdb":{"type":"literal","value":"67744"}}]}}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.endpoint = srv.URL + "/sparql"

	tmdb, err := client.TMDBIDByTVDB(t.Context(), 328708)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if tmdb == nil || *tmdb != 67744 {
		t.Fatalf("unexpected tmdb id: %v", tmdb)
	}
}

func TestTMDBIDByIMDB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")
		if !strings.Contains(query, `wdt:P345 "tt5290382"`) {
			t.Fatalf("unexpected query: %s", query)
		}
		_, _ = w.Write([]byte(`{
			"results": {
				"bindings": [{
					"show": {"type": "uri", "value": "http://www.wikidata.org/entity/Q25340152"},
					"tmdb": {"type": "literal", "value": "67744"}
				}]
			}
		}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.endpoint = srv.URL + "/sparql"

	tmdb, err := client.TMDBIDByIMDB(t.Context(), "tt5290382")
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 67744 {
		t.Fatalf("unexpected tmdb id: %v", tmdb)
	}
}

func TestTMDBIDByIMDBRejectsInvalidID(t *testing.T) {
	t.Parallel()

	client := NewClient()
	cases := []string{
		`tt1" . ?show wdt:P345 "tt2`,
		"ttabc",
		`tt123; DROP`,
	}
	for _, imdbID := range cases {
		tmdb, err := client.TMDBIDByIMDB(t.Context(), imdbID)
		if err != nil {
			t.Fatalf("imdbID %q: %v", imdbID, err)
		}
		if tmdb != nil {
			t.Fatalf("imdbID %q: expected nil, got %d", imdbID, *tmdb)
		}
	}
}

func TestNormalizeIMDBID(t *testing.T) {
	t.Parallel()

	if got := normalizeIMDBID("5290382"); got != "tt5290382" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeIMDBID(`tt1"evil`); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
