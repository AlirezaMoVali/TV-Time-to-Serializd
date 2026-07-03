package service

import (
	"fmt"
	"strings"

	"github.com/alireza/tvtime2serializd/internal/serializd"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
)

const (
	tvStatusStopped       = "stopped"
	tvStatusUpToDate      = "up_to_date"
	tvStatusNotStartedYet = "not_started_yet"
	tvStatusContinuing    = "continuing"
	tvStatusWatchLater    = "watch_later"
)

type seasonState int

const (
	seasonUnwatched seasonState = iota
	seasonPartial
	seasonFullyWatched
)

type serializdShowImporter interface {
	GetShow(token string, showID int) (*serializd.Show, error)
	AddWatchlist(token string, showID int, seasonIDs []int) error
	AddCurrentlyWatching(token string, showID int) (*serializd.MessageResponse, error)
	AddWatched(token string, showID int, seasonIDs []int, async bool) (*serializd.MessageResponse, error)
	AddDropped(token string, showID int) (*serializd.MessageResponse, error)
	AddEpisodeLog(token string, showID, seasonID int, episodeNumbers []int, shouldGetNextEpisode bool) error
}

type mappedSeason struct {
	szSeason        serializd.Season
	isSpecial       bool
	state           seasonState
	watchedEpisodes []int
}

// ImportTVTimeShow maps a TV Time series export onto Serializd tracking state.
func ImportTVTimeShow(client serializdShowImporter, token string, tmdbID int, show tvtime.ExportShow) error {
	szShow, err := client.GetShow(token, tmdbID)
	if err != nil {
		return err
	}

	regularSeasonIDs := serializd.RegularSeasonIDs(szShow.Seasons)
	if len(regularSeasonIDs) == 0 {
		return fmt.Errorf("no regular seasons found for show %d", tmdbID)
	}

	seasons := mapSeasons(szShow, show)
	status := strings.ToLower(strings.TrimSpace(show.Status))

	switch status {
	case tvStatusNotStartedYet, tvStatusWatchLater, "":
		return client.AddWatchlist(token, tmdbID, regularSeasonIDs)

	case tvStatusUpToDate:
		if _, err := client.AddWatched(token, tmdbID, regularSeasonIDs, true); err != nil {
			return err
		}
		return logWatchedEpisodes(client, token, tmdbID, seasons)

	case tvStatusContinuing:
		if _, err := client.AddCurrentlyWatching(token, tmdbID); err != nil {
			return err
		}
		return applyContinuing(client, token, tmdbID, seasons)

	case tvStatusStopped:
		if err := applyStoppedWatched(client, token, tmdbID, seasons); err != nil {
			return err
		}
		_, err := client.AddDropped(token, tmdbID)
		return err

	default:
		return fmt.Errorf("unsupported TV Time status %q", show.Status)
	}
}

func mapSeasons(szShow *serializd.Show, show tvtime.ExportShow) []mappedSeason {
	tvByKey := make(map[string]tvtime.ExportSeason, len(show.Seasons))
	for _, season := range show.Seasons {
		key := seasonKey(season.Number, tvtime.IsSpecialSeason(season))
		tvByKey[key] = season
	}

	out := make([]mappedSeason, 0, len(szShow.Seasons))
	for _, szSeason := range szShow.Seasons {
		isSpecial := serializd.IsSpecialSeason(szSeason)
		if isSpecial && szSeason.SeasonNumber != 0 {
			// regular season whose name contains "special" — still a regular season
			isSpecial = false
		}
		key := seasonKey(szSeason.SeasonNumber, isSpecial)
		tvSeason, ok := tvByKey[key]
		watched, state := classifyTVSeason(tvSeason, ok)
		out = append(out, mappedSeason{
			szSeason:        szSeason,
			isSpecial:       isSpecial,
			state:           state,
			watchedEpisodes: watched,
		})
	}
	return out
}

func seasonKey(number int, isSpecial bool) string {
	if isSpecial {
		return "special"
	}
	return fmt.Sprintf("s%d", number)
}

func classifyTVSeason(tvSeason tvtime.ExportSeason, hasData bool) ([]int, seasonState) {
	if !hasData {
		return nil, seasonUnwatched
	}
	trackable := trackableEpisodes(tvSeason.Episodes)
	if len(trackable) == 0 {
		return nil, seasonUnwatched
	}
	watched := watchedEpisodeNumbers(trackable)
	switch {
	case len(watched) == 0:
		return nil, seasonUnwatched
	case len(watched) == len(trackable):
		return watched, seasonFullyWatched
	default:
		return watched, seasonPartial
	}
}

func trackableEpisodes(episodes []tvtime.ExportEpisode) []tvtime.ExportEpisode {
	out := make([]tvtime.ExportEpisode, 0, len(episodes))
	for _, ep := range episodes {
		if ep.Number > 0 {
			out = append(out, ep)
		}
	}
	return out
}

func applyContinuing(client serializdShowImporter, token string, tmdbID int, seasons []mappedSeason) error {
	watchlistIDs := make([]int, 0)
	watchedIDs := make([]int, 0)

	for _, season := range seasons {
		if season.isSpecial {
			continue
		}
		id := season.szSeason.SeasonIDValue()
		switch season.state {
		case seasonFullyWatched:
			watchedIDs = append(watchedIDs, id)
		case seasonPartial, seasonUnwatched:
			watchlistIDs = append(watchlistIDs, id)
		}
	}

	if len(watchlistIDs) > 0 {
		if err := client.AddWatchlist(token, tmdbID, watchlistIDs); err != nil {
			return err
		}
	}
	if len(watchedIDs) > 0 {
		if _, err := client.AddWatched(token, tmdbID, watchedIDs, true); err != nil {
			return err
		}
	}
	return logWatchedEpisodes(client, token, tmdbID, seasons)
}

func applyStoppedWatched(client serializdShowImporter, token string, tmdbID int, seasons []mappedSeason) error {
	watchedIDs := make([]int, 0)
	for _, season := range seasons {
		if season.state == seasonFullyWatched && !season.isSpecial {
			watchedIDs = append(watchedIDs, season.szSeason.SeasonIDValue())
		}
	}
	if len(watchedIDs) > 0 {
		if _, err := client.AddWatched(token, tmdbID, watchedIDs, true); err != nil {
			return err
		}
	}
	return logWatchedEpisodes(client, token, tmdbID, seasons)
}

// logWatchedEpisodes marks individual episodes in Serializd. AddWatched only tracks
// the season; episode logs are required for per-episode watched state in the UI.
func logWatchedEpisodes(client serializdShowImporter, token string, tmdbID int, seasons []mappedSeason) error {
	for _, season := range seasons {
		if len(season.watchedEpisodes) == 0 {
			continue
		}
		if err := client.AddEpisodeLog(token, tmdbID, season.szSeason.SeasonIDValue(), season.watchedEpisodes, false); err != nil {
			label := fmt.Sprintf("season %d", season.szSeason.SeasonNumber)
			if season.isSpecial {
				label = "specials"
			}
			return fmt.Errorf("episode log %s: %w", label, err)
		}
	}
	return nil
}

func watchedEpisodeNumbers(episodes []tvtime.ExportEpisode) []int {
	out := make([]int, 0, len(episodes))
	for _, ep := range episodes {
		if ep.IsWatched && ep.Number > 0 {
			out = append(out, ep.Number)
		}
	}
	return out
}
