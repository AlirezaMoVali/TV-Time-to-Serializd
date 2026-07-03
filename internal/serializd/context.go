package serializd

import (
	"encoding/json"

	"github.com/alireza/tvtime2serializd/internal/safenum"
)

// ExtractTrackedShowIDs collects TMDB show IDs from Serializd user context.
func ExtractTrackedShowIDs(context json.RawMessage) map[int]struct{} {
	ids := make(map[int]struct{})
	if len(context) == 0 {
		return ids
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(context, &root); err != nil {
		collectShowIDs(context, ids)
		return ids
	}

	for _, raw := range root {
		collectShowIDs(raw, ids)
	}
	return ids
}

func collectShowIDs(raw json.RawMessage, ids map[int]struct{}) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			collectShowIDs(item, ids)
		}
		return
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return
	}

	for _, key := range []string{"show_id", "showId", "id", "tmdb_id", "tmdbId"} {
		if v, ok := obj[key]; ok {
			if id, ok := asInt(v); ok {
				ids[id] = struct{}{}
			}
		}
	}

	for _, v := range obj {
		switch child := v.(type) {
		case map[string]any:
			b, _ := json.Marshal(child)
			collectShowIDs(b, ids)
		case []any:
			b, _ := json.Marshal(child)
			collectShowIDs(b, ids)
		}
	}
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if i, ok := safenum.Float64ToInt(n); ok {
			return i, i > 0
		}
		return 0, false
	case int:
		return n, n > 0
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		if out, ok := safenum.Float64ToInt(float64(i)); ok {
			return out, out > 0
		}
		return 0, false
	default:
		return 0, false
	}
}

// RegularSeasonIDs returns non-special season IDs for watchlist import.
func RegularSeasonIDs(seasons []Season) []int {
	out := make([]int, 0, len(seasons))
	for _, season := range seasons {
		if season.SeasonNumber == 0 {
			continue
		}
		id := season.SeasonIDValue()
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}
