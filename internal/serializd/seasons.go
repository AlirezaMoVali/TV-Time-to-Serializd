package serializd

import "strings"

// IsSpecialSeason reports whether a Serializd season is a specials bucket.
func IsSpecialSeason(season Season) bool {
	if season.SeasonNumber == 0 {
		return true
	}
	return strings.Contains(strings.ToLower(season.Name), "special")
}

// MatchSeason finds the Serializd season for a TV Time season number.
func MatchSeason(seasons []Season, seasonNumber int, isSpecial bool) (Season, bool) {
	if isSpecial {
		for _, season := range seasons {
			if IsSpecialSeason(season) {
				return season, true
			}
		}
		return Season{}, false
	}

	for _, season := range seasons {
		if season.SeasonNumber == seasonNumber && !IsSpecialSeason(season) {
			return season, true
		}
	}
	return Season{}, false
}
