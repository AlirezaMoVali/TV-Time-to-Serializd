package tvtime

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alireza/tvtime2serializd/internal/applog"
)

const (
	cgwFollowsBase = "https://msapi.tvtime.com/prod/v1/tracking/cgw/follows/user/"
	watchesBase    = "https://msapi.tvtime.com/prod/v1/tracking/watches/user/"
	seriesBase     = "https://msapi.tvtime.com/v1/series/"
)

func (c *Client) ExportShows(tokens *Tokens) (*ExportResult, error) {
	return c.exportShowsInternal(tokens, nil)
}

func (c *Client) ExportShowsWithProgress(tokens *Tokens, onGather GatherProgressFunc) (*ExportResult, error) {
	return c.exportShowsInternal(tokens, onGather)
}

func (c *Client) exportShowsInternal(tokens *Tokens, onGather GatherProgressFunc) (*ExportResult, error) {
	if err := ValidateTokens(tokens); err != nil {
		return nil, err
	}

	start := time.Now()
	userID := c.userID(tokens)
	token := c.activeJWT(tokens)

	cgwURL := cgwFollowsBase + strconv.FormatInt(userID, 10)
	watchesURL := watchesBase + strconv.FormatInt(userID, 10)

	report := func(p GatherProgress) {
		if onGather != nil {
			onGather(p)
		}
	}

	report(GatherProgress{Phase: GatherPhaseFollows, Done: 0, Total: 2, Detail: "Fetching followed series and anime from TV Time…"})

	var (
		seriesRaw []map[string]any
		animeRaw  []map[string]any
		seriesErr error
		animeErr  error
		wg        sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		seriesRaw, seriesErr = c.fetchObjectsWithRetry(token, cgwURL, "series", 1000)
	}()
	go func() {
		defer wg.Done()
		animeRaw, animeErr = c.fetchObjectsWithRetry(token, cgwURL, "anime", 1000)
	}()
	wg.Wait()
	if seriesErr != nil {
		return nil, fmt.Errorf("fetch series: %w", seriesErr)
	}
	if animeErr != nil {
		return nil, fmt.Errorf("fetch anime: %w", animeErr)
	}
	report(GatherProgress{Phase: GatherPhaseFollows, Done: 1, Total: 2, Detail: fmt.Sprintf("Fetched %d series", len(seriesRaw))})
	report(GatherProgress{Phase: GatherPhaseFollows, Done: 2, Total: 2, Detail: fmt.Sprintf("Fetched %d anime", len(animeRaw))})

	showsRaw := append(seriesRaw, animeRaw...)
	followedUUIDs := make(map[string]struct{}, len(showsRaw))
	shows := make([]followShow, 0, len(showsRaw))

	for _, raw := range showsRaw {
		show := parseFollowShow(raw)
		if show.UUID != "" {
			followedUUIDs[show.UUID] = struct{}{}
		}
		shows = append(shows, show)
	}
	report(GatherProgress{Phase: GatherPhaseFollows, Done: 2, Total: 2, Detail: fmt.Sprintf("Found %d followed shows", len(shows))})

	// Refresh JWT before watch history — Liberator does the same at this boundary.
	token = c.activeJWT(tokens)
	report(GatherProgress{Phase: GatherPhaseWatches, Done: 0, Total: 0, Detail: "Fetching watch history from TV Time…"})
	episodeWatches, err := c.fetchObjects(token, watchesURL, "episode", 100, func(fetched, page int) {
		report(GatherProgress{
			Phase:  GatherPhaseWatches,
			Done:   fetched,
			Total:  0,
			Detail: fmt.Sprintf("Watch history page %d (%d records)", page, fetched),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetch episode watches: %w", err)
	}
	applog.Info("tvtime episode watches fetched", "count", len(episodeWatches))
	report(GatherProgress{Phase: GatherPhaseWatches, Done: len(episodeWatches), Total: 0, Detail: fmt.Sprintf("Fetched %d watch records", len(episodeWatches))})

	watchedAtMap := buildWatchedAtMap(episodeWatches, followedUUIDs)

	if err := c.fetchAllSeasons(token, shows, onGather); err != nil {
		return nil, err
	}

	exportShows := normalizeShows(shows, watchedAtMap)
	seriesIDs := make(map[string]int64, len(shows))
	for _, show := range shows {
		if show.UUID != "" && show.SeriesID != 0 {
			seriesIDs[show.UUID] = show.SeriesID
		}
	}
	return &ExportResult{
		Shows:           exportShows,
		SeriesIDs:       seriesIDs,
		WatchedEpisodes: countWatchedEpisodes(exportShows),
		DurationMs:      time.Since(start).Milliseconds(),
	}, nil
}

func (c *Client) fetchAllSeasons(token string, shows []followShow, onGather GatherProgressFunc) error {
	total := len(shows)
	if total == 0 {
		return nil
	}

	workers := gatherConcurrency()
	if workers > total {
		workers = total
	}

	httpClient := c.paginatedHTTP()

	if onGather != nil {
		onGather(GatherProgress{Phase: GatherPhaseEpisodes, Done: 0, Total: total, Detail: "Fetching episode details…"})
	}

	jobs := make(chan int, workers)
	var wg sync.WaitGroup
	var done atomic.Int32

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				show := &shows[i]
				if show.SeriesID == 0 {
					show.Seasons = []ExportSeason{}
				} else {
					seasons, year, err := c.fetchSeriesEpisodes(httpClient, token, show.SeriesID)
					if err != nil {
						seasons, year, err = c.fetchSeriesEpisodes(httpClient, token, show.SeriesID)
					}
					if err != nil {
						show.Seasons = []ExportSeason{}
						show.Ref["_noEpisodeData"] = true
					} else {
						show.Seasons = seasons
						if show.Year == nil {
							show.Year = year
						}
						if show.Year == nil {
							show.Year = c.fetchShowYear(httpClient, token, show.SeriesID)
						}
					}
				}

				n := int(done.Add(1))
				if onGather != nil {
					showTitle := show.Title
					detail := fmt.Sprintf("Fetching episode details (%d/%d)", n, total)
					if showTitle != "" {
						detail = fmt.Sprintf("Fetching episodes: %s (%d/%d)", showTitle, n, total)
					}
					onGather(GatherProgress{
						Phase:    GatherPhaseEpisodes,
						Done:     n,
						Total:    total,
						Detail:   detail,
						ShowName: showTitle,
					})
				}
			}
		}()
	}

	for i := range shows {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return nil
}

func (c *Client) fetchSeriesEpisodes(httpClient *http.Client, token string, seriesID int64) ([]ExportSeason, *int, error) {
	innerURL := seriesBase + strconv.FormatInt(seriesID, 10) + "/episodes"
	raw, err := c.sidecarWithClient(httpClient, innerURL, http.MethodGet, nil, token)
	if err != nil {
		return nil, nil, err
	}

	data, ok := raw["data"].([]any)
	if !ok {
		return nil, parseYearFromEpisodesRaw(raw), fmt.Errorf("episodes response missing data array")
	}

	seasonMap := make(map[int]*ExportSeason)
	for _, item := range data {
		ep, ok := item.(map[string]any)
		if !ok {
			continue
		}
		seasonNum := 0
		if season, ok := ep["season"].(map[string]any); ok {
			seasonNum = asInt(season["number"])
		}
		s, exists := seasonMap[seasonNum]
		if !exists {
			s = &ExportSeason{
				Number:     seasonNum,
				IsSpecials: seasonNum == 0,
				Episodes:   []ExportEpisode{},
			}
			seasonMap[seasonNum] = s
		}

		epID := asInt64(ep["id"])
		name, _ := ep["name"].(string)
		if name == "" {
			name, _ = ep["title"].(string)
		}
		isSpecial, _ := ep["is_special"].(bool)
		s.Episodes = append(s.Episodes, ExportEpisode{
			ID:      ExternalIDs{TVDB: int64Ptr(epID)},
			Number:  asInt(ep["number"]),
			Name:    strPtr(name),
			Special: isSpecial || seasonNum == 0,
		})
	}

	seasons := make([]ExportSeason, 0, len(seasonMap))
	for _, s := range seasonMap {
		seasons = append(seasons, *s)
	}
	sortSeasons(seasons)
	return seasons, parseYearFromEpisodesRaw(raw), nil
}

func parseYearFromEpisodesRaw(raw map[string]any) *int {
	if data, ok := raw["data"].(map[string]any); ok {
		if year := parseShowYear(data); year != nil {
			return year
		}
	}
	return parseShowYear(raw)
}

func (c *Client) fetchShowYear(httpClient *http.Client, token string, seriesID int64) *int {
	innerURL := seriesBase + strconv.FormatInt(seriesID, 10)
	raw, err := c.sidecarWithClient(httpClient, innerURL, http.MethodGet, nil, token)
	if err != nil {
		return nil
	}
	if data, ok := raw["data"].(map[string]any); ok {
		return parseShowYear(data)
	}
	return parseShowYear(raw)
}

func sortSeasons(seasons []ExportSeason) {
	slices.SortFunc(seasons, func(a, b ExportSeason) int {
		return cmp.Compare(a.Number, b.Number)
	})
}

func (c *Client) sidecarWithClient(httpClient *http.Client, targetURL, method string, body any, bearer string) (map[string]any, error) {
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.apiSidecarBase()+"?o_b64="+b64url(targetURL), payload)
	if err != nil {
		return nil, err
	}
	setSidecarHeaders(req, bearer)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.roundTrip(context.Background(), httpClient, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s failed (%d): %s", method, targetURL, resp.StatusCode, raw)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func setSidecarHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://app.tvtime.com")
	req.Header.Set("Referer", "https://app.tvtime.com/email")
	req.Header.Set("app-version", "2025082201")
	req.Header.Set("client-version", "10.10.0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tvtime2serializd/1.0)")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func extractObjects(raw map[string]any) []map[string]any {
	data, ok := raw["data"].(map[string]any)
	if !ok {
		return nil
	}
	items, ok := data["objects"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func parseFollowShow(raw map[string]any) followShow {
	show := followShow{Ref: raw, Status: "unknown"}
	if uuid, ok := raw["uuid"].(string); ok {
		show.UUID = uuid
	}
	show.CreatedAt = parseCreatedAt(raw["created_at"])

	if meta, ok := raw["meta"].(map[string]any); ok {
		show.SeriesID = asInt64(meta["id"])
		if show.SeriesID != 0 {
			show.TVDBID = int64Ptr(show.SeriesID)
		}
		if name, ok := meta["name"].(string); ok {
			show.Title = name
		} else if title, ok := meta["title"].(string); ok {
			show.Title = title
		}
		show.Year = parseShowYear(meta)
	}

	if filter, ok := raw["filter"].([]any); ok && len(filter) > 1 {
		if status, ok := filter[1].(string); ok {
			show.Status = status
		}
	}
	return show
}

func buildWatchedAtMap(watches []map[string]any, followedUUIDs map[string]struct{}) map[string]watchEntry {
	out := make(map[string]watchEntry)
	for _, w := range watches {
		seriesUUID, _ := w["series_uuid"].(string)
		if seriesUUID == "" {
			continue
		}
		if _, ok := followedUUIDs[seriesUUID]; !ok {
			continue
		}
		entry := watchEntry{
			WatchedAt:    formatWatchedAt(w["watched_at"]),
			RewatchCount: asInt(w["rewatch_count"]),
		}
		if key := watchMapKey(w["episode_id"]); key != "" {
			out[key] = entry
		}
		if key := watchMapKey(w["uuid"]); key != "" {
			out[key] = entry
		}
	}
	return out
}

func normalizeShows(shows []followShow, watchedAtMap map[string]watchEntry) []ExportShow {
	out := make([]ExportShow, 0, len(shows))
	for _, show := range shows {
		tvdbID := show.TVDBID

		noEpisodeData := false
		if v, ok := show.Ref["_noEpisodeData"].(bool); ok {
			noEpisodeData = v
		}

		exportShow := ExportShow{
			UUID:          strPtr(show.UUID),
			ID:            ExternalIDs{TVDB: tvdbID, IMDB: nil},
			CreatedAt:     show.CreatedAt,
			Title:         strPtr(show.Title),
			Year:          show.Year,
			Status:        show.Status,
			IsFavorite:    false,
			NoEpisodeData: noEpisodeData,
			Seasons:       make([]ExportSeason, 0, len(show.Seasons)),
		}

		for _, season := range show.Seasons {
			exportSeason := ExportSeason{
				Number:     season.Number,
				IsSpecials: season.Number == 0,
				Episodes:   make([]ExportEpisode, 0, len(season.Episodes)),
			}
			for _, ep := range season.Episodes {
				name := ""
				if ep.Name != nil {
					name = strings.TrimSpace(*ep.Name)
				}
				if strings.EqualFold(name, "TBA") {
					continue
				}

				var epKey string
				if ep.ID.TVDB != nil {
					epKey = watchMapKey(*ep.ID.TVDB)
				}
				entry, hasWatch := watchedAtMap[epKey]
				isWatched := hasWatch || ep.IsWatched
				var watchedAt *string
				rewatchCount := 0
				if hasWatch {
					watchedAt = entry.WatchedAt
					rewatchCount = entry.RewatchCount
				}

				exportSeason.Episodes = append(exportSeason.Episodes, ExportEpisode{
					ID:           ep.ID,
					Number:       ep.Number,
					Name:         ep.Name,
					Special:      ep.Special || season.Number == 0,
					IsWatched:    isWatched,
					WatchedAt:    watchedAt,
					RewatchCount: rewatchCount,
					WatchedCount: episodeWatchedCount(isWatched, hasWatch, rewatchCount),
				})
			}
			exportShow.Seasons = append(exportShow.Seasons, exportSeason)
		}
		out = append(out, exportShow)
	}
	return out
}
