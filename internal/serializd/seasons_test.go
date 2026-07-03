package serializd

import "testing"

func TestIsSpecialSeason(t *testing.T) {
	if !IsSpecialSeason(Season{SeasonNumber: 0, Name: "Season 0"}) {
		t.Fatal("season 0 should be special")
	}
	if !IsSpecialSeason(Season{SeasonNumber: 1, Name: "Specials"}) {
		t.Fatal("name containing special should match")
	}
	if IsSpecialSeason(Season{SeasonNumber: 1, Name: "Season 1"}) {
		t.Fatal("regular season should not be special")
	}
}

func TestMatchSeason(t *testing.T) {
	seasons := []Season{
		{SeasonID: 1, SeasonNumber: 1, Name: "Season 1"},
		{SeasonID: 99, SeasonNumber: 0, Name: "Specials"},
	}

	got, ok := MatchSeason(seasons, 1, false)
	if !ok || got.SeasonID != 1 {
		t.Fatalf("regular match = %+v, ok=%v", got, ok)
	}

	got, ok = MatchSeason(seasons, 0, true)
	if !ok || got.SeasonID != 99 {
		t.Fatalf("special match = %+v, ok=%v", got, ok)
	}
}
