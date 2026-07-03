package tvtime

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestShowsToJSONMatchesLiberatorShape(t *testing.T) {
	title := "MINDHUNTER"
	watched := "2018-07-01 06:46:41"
	tvdb := int64(328708)
	epTVDB := int64(6124070)
	shows := []ExportShow{{
		UUID:       strPtr("dc085400-dbec-4155-9042-af18032ceece"),
		ID:         ExternalIDs{TVDB: &tvdb},
		Title:      &title,
		Status:     "continuing",
		IsFavorite: false,
		Seasons: []ExportSeason{{
			Number:     1,
			IsSpecials: false,
			Episodes: []ExportEpisode{{
				ID:           ExternalIDs{TVDB: &epTVDB},
				Number:       1,
				Name:         strPtr("Episode 1"),
				IsWatched:    true,
				WatchedAt:    &watched,
				RewatchCount: 0,
				WatchedCount: 1,
			}},
		}},
	}}

	data, err := ShowsToJSON(shows)
	if err != nil {
		t.Fatal(err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected array of 1 show, got %d", len(decoded))
	}
	if decoded[0]["title"] != title {
		t.Fatalf("unexpected title: %v", decoded[0]["title"])
	}
}

func TestShowsToCSVHeaderAndRow(t *testing.T) {
	title := "MINDHUNTER"
	tvdb := int64(328708)
	epTVDB := int64(6124070)
	shows := []ExportShow{{
		UUID:   strPtr("uuid-1"),
		ID:     ExternalIDs{TVDB: &tvdb},
		Title:  &title,
		Status: "continuing",
		Seasons: []ExportSeason{{
			Number: 1,
			Episodes: []ExportEpisode{{
				ID:        ExternalIDs{TVDB: &epTVDB},
				Number:    1,
				Name:      strPtr("Episode 1"),
				IsWatched: true,
			}},
		}},
	}}

	data, err := ShowsToCSV(shows)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "uuid,title,show_tvdb") {
		t.Fatalf("missing csv header: %s", text)
	}
	if !strings.Contains(text, "uuid-1,MINDHUNTER,328708") {
		t.Fatalf("missing csv row: %s", text)
	}
}
