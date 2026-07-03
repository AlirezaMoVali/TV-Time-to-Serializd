package tvtime

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alireza/tvtime2serializd/internal/safenum"
)

// ExportShow matches the TV Time Liberator JSON format for series.
type ExportShow struct {
	UUID           *string         `json:"uuid"`
	ID             ExternalIDs     `json:"id"`
	CreatedAt      *string         `json:"created_at"`
	Title          *string         `json:"title"`
	Year           *int            `json:"year,omitempty"`
	Status         string          `json:"status"`
	IsFavorite     bool            `json:"is_favorite"`
	NoEpisodeData  bool            `json:"_noEpisodeData"`
	Seasons        []ExportSeason  `json:"seasons"`
}

type ExternalIDs struct {
	TVDB *int64  `json:"tvdb"`
	IMDB *string `json:"imdb"`
}

type ExportSeason struct {
	Number     int             `json:"number"`
	IsSpecials bool            `json:"is_specials"`
	Episodes   []ExportEpisode `json:"episodes"`
}

type ExportEpisode struct {
	ID           ExternalIDs `json:"id"`
	Number       int         `json:"number"`
	Name         *string     `json:"name"`
	Special      bool        `json:"special"`
	IsWatched    bool        `json:"is_watched"`
	WatchedAt    *string     `json:"watched_at"`
	RewatchCount int         `json:"rewatch_count"`
	WatchedCount int         `json:"watched_count"`
}

type ExportResult struct {
	Shows           []ExportShow       `json:"shows"`
	SeriesIDs       map[string]int64   `json:"-"`
	WatchedEpisodes int                `json:"watched_episodes"`
	DurationMs      int64              `json:"duration_ms"`
}

type watchEntry struct {
	WatchedAt    *string
	RewatchCount int
}

type followShow struct {
	UUID      string
	CreatedAt *string
	Year      *int
	Status    string
	SeriesID  int64
	TVDBID    *int64
	Title     string
	Ref       map[string]any
	Seasons   []ExportSeason
}

func int64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func formatWatchedAt(raw any) *string {
	if raw == nil {
		return nil
	}
	s, ok := raw.(string)
	if !ok || s == "" {
		return nil
	}
	// Match TV Time Liberator: "YYYY-MM-DD HH:MM:SS"
	s = strings.Replace(s, "T", " ", 1)
	if idx := strings.Index(s, "."); idx > 0 {
		s = s[:idx]
	}
	s = strings.TrimSuffix(s, "Z")
	if len(s) >= 19 {
		out := s[:19]
		return &out
	}
	return nil
}

func parseCreatedAt(raw any) *string {
	s, ok := raw.(string)
	if !ok || s == "" {
		return nil
	}
	return &s
}

// parseShowYear extracts a premiere/release year from TV Time show metadata.
func parseShowYear(raw map[string]any) *int {
	if raw == nil {
		return nil
	}
	for _, key := range []string{"year", "release_year", "start_year"} {
		if year := intFromAny(raw[key]); year != nil {
			return year
		}
	}
	for _, key := range []string{"first_aired", "first_air_date", "aired_at", "premiere_date", "release_date"} {
		if year := yearFromDateValue(raw[key]); year != nil {
			return year
		}
	}
	return nil
}

func intFromAny(raw any) *int {
	switch v := raw.(type) {
	case float64:
		if i, ok := safenum.Float64ToInt(v); ok && i > 0 {
			return &i
		}
	case int:
		if v > 0 {
			return &v
		}
	case int64:
		if i, ok := safenum.Float64ToInt(float64(v)); ok && i > 0 {
			return &i
		}
	case string:
		if len(v) >= 4 {
			if y, err := strconv.Atoi(v[:4]); err == nil && y > 0 {
				return &y
			}
		}
	}
	return nil
}

func yearFromDateValue(raw any) *int {
	s, ok := raw.(string)
	if !ok || len(s) < 4 {
		return nil
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil || y <= 0 {
		return nil
	}
	return &y
}

func episodeWatchedCount(watched bool, hasWatch bool, rewatchCount int) int {
	if hasWatch {
		return rewatchCount + 1
	}
	if watched {
		return 1
	}
	return 0
}

func countWatchedEpisodes(shows []ExportShow) int {
	total := 0
	for _, show := range shows {
		for _, season := range show.Seasons {
			if season.Number == 0 {
				continue
			}
			for _, ep := range season.Episodes {
				if ep.IsWatched && !ep.Special {
					total++
				}
			}
		}
	}
	return total
}

// PtrTimeRFC3339 formats a time pointer for JSON output.
func PtrTimeRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// FormatLiberatorDateTime formats watched_at like TV Time Liberator exports.
func FormatLiberatorDateTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02 15:04:05")
	return &s
}

// OutputFormat is the requested download format for an export.
type OutputFormat string

const (
	OutputFormatJSON OutputFormat = "json"
	OutputFormatCSV  OutputFormat = "csv"
	OutputFormatBoth OutputFormat = "both"
)

func ParseOutputFormat(raw string) (OutputFormat, error) {
	switch OutputFormat(raw) {
	case OutputFormatJSON, OutputFormatCSV, OutputFormatBoth, "":
		if raw == "" {
			return OutputFormatJSON, nil
		}
		return OutputFormat(raw), nil
	default:
		return "", fmt.Errorf("format must be json, csv, or both")
	}
}
