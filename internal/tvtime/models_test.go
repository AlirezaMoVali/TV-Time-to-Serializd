package tvtime

import "testing"

func TestParseShowYear(t *testing.T) {
	year := 2017
	got := parseShowYear(map[string]any{"year": float64(2017)})
	if got == nil || *got != year {
		t.Fatalf("expected 2017, got %v", got)
	}

	got = parseShowYear(map[string]any{"first_aired": "2017-10-13"})
	if got == nil || *got != year {
		t.Fatalf("expected 2017 from date, got %v", got)
	}

	if parseShowYear(nil) != nil {
		t.Fatal("expected nil for nil meta")
	}
}

func TestParseFollowShowYear(t *testing.T) {
	show := parseFollowShow(map[string]any{
		"uuid": "abc",
		"meta": map[string]any{
			"id":   float64(328708),
			"name": "Mindhunter",
			"year": float64(2017),
		},
	})
	if show.Year == nil || *show.Year != 2017 {
		t.Fatalf("expected year 2017, got %v", show.Year)
	}
}
