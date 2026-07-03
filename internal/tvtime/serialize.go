package tvtime

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
)

// ShowsToJSON encodes shows as a top-level JSON array matching the TV Time Liberator export file.
func ShowsToJSON(shows []ExportShow) ([]byte, error) {
	return json.MarshalIndent(shows, "", "  ")
}

// ShowsToCSV flattens shows into one row per episode (Liberator-compatible CSV).
func ShowsToCSV(shows []ExportShow) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	header := []string{
		"uuid", "title", "show_tvdb", "show_imdb", "year", "status", "is_favorite", "created_at",
		"season_number", "is_specials", "episode_number", "episode_tvdb", "episode_imdb",
		"episode_name", "is_special", "is_watched", "watched_at", "rewatch_count", "watched_count",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, show := range shows {
		for _, season := range show.Seasons {
			for _, ep := range season.Episodes {
				row := []string{
					derefString(show.UUID),
					derefString(show.Title),
					formatInt64Ptr(show.ID.TVDB),
					derefString(show.ID.IMDB),
					formatIntPtr(show.Year),
					show.Status,
					strconv.FormatBool(show.IsFavorite),
					derefString(show.CreatedAt),
					strconv.Itoa(season.Number),
					strconv.FormatBool(season.IsSpecials),
					strconv.Itoa(ep.Number),
					formatInt64Ptr(ep.ID.TVDB),
					derefString(ep.ID.IMDB),
					derefString(ep.Name),
					strconv.FormatBool(ep.Special),
					strconv.FormatBool(ep.IsWatched),
					derefString(ep.WatchedAt),
					strconv.Itoa(ep.RewatchCount),
					strconv.Itoa(ep.WatchedCount),
				}
				if err := w.Write(row); err != nil {
					return nil, fmt.Errorf("write csv row: %w", err)
				}
			}
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func formatInt64Ptr(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func formatIntPtr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
