package serializd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTMDBIDByTitleYear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/search/shows") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query().Get("search_query")
		if query != "MINDHUNTER (2017)" {
			t.Fatalf("unexpected search query: %s", query)
		}
		_, _ = w.Write([]byte(`{
			"results": [
				{"id":67744,"name":"MINDHUNTER","firstAirDate":"2017-10-13"},
				{"id":222508,"name":"Violent Minds: Killers on Tape","firstAirDate":"2023-04-02"}
			],
			"totalPages": 1
		}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	year := 2017
	tmdb, err := client.TMDBIDByTitleYear("MINDHUNTER", &year)
	if err != nil {
		t.Fatal(err)
	}
	if tmdb == nil || *tmdb != 67744 {
		t.Fatalf("unexpected tmdb id: %v", tmdb)
	}
}

func TestPickShowSearchResultYearMismatchSkipsWrongTitle(t *testing.T) {
	year := 2017
	results := []ShowSearchResult{
		{ID: 222508, Name: "Other Show", FirstAirDate: "2017-01-01"},
	}
	if pick := pickShowSearchResult(results, "MINDHUNTER", &year); pick != nil {
		t.Fatalf("expected nil, got %d", *pick)
	}
}
