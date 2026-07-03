package serializd

import "encoding/json"

type LoginResponse struct {
	Token string `json:"token"`
}

type Show struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Seasons []Season `json:"seasons"`
}

type Season struct {
	ID           int    `json:"id"`
	SeasonID     int    `json:"seasonId"`
	SeasonNumber int    `json:"seasonNumber"`
	Name         string `json:"name"`
	EpisodeCount int    `json:"episodeCount"`
}

// SeasonID returns the TMDB season ID used by watchlist/watched endpoints.
func (s Season) SeasonIDValue() int {
	if s.SeasonID != 0 {
		return s.SeasonID
	}
	return s.ID
}

type UserInformation struct {
	Context json.RawMessage `json:"context"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
