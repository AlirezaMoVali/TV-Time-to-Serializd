package serializd

import (
	"encoding/json"
	"testing"
)

func TestExtractTrackedShowIDs(t *testing.T) {
	raw := json.RawMessage(`{
		"watchlist": [{"show_id": 67744, "name": "Mindhunter"}],
		"watched": [{"id": 1399}]
	}`)
	ids := ExtractTrackedShowIDs(raw)
	if _, ok := ids[67744]; !ok {
		t.Fatalf("expected 67744 in ids: %v", ids)
	}
	if _, ok := ids[1399]; !ok {
		t.Fatalf("expected 1399 in ids: %v", ids)
	}
}

func TestRegularSeasonIDs(t *testing.T) {
	seasons := []Season{
		{SeasonNumber: 0, ID: 1},
		{SeasonNumber: 1, SeasonID: 3624},
	}
	ids := RegularSeasonIDs(seasons)
	if len(ids) != 1 || ids[0] != 3624 {
		t.Fatalf("unexpected season ids: %v", ids)
	}
}
