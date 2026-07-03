package tvtime

// IsSpecialSeason reports whether a TV Time export season is a specials bucket.
func IsSpecialSeason(season ExportSeason) bool {
	return season.IsSpecials || season.Number == 0
}
