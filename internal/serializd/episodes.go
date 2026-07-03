package serializd

type episodeLogBody struct {
	ShowID               int   `json:"show_id"`
	SeasonID             int   `json:"season_id"`
	EpisodeNumbers       []int `json:"episode_numbers"`
	ShouldGetNextEpisode bool  `json:"should_get_next_episode"`
}

func (c *Client) AddEpisodeLog(token string, showID, seasonID int, episodeNumbers []int, shouldGetNextEpisode bool) error {
	return c.post("/episode_log/add", episodeLogBody{
		ShowID:               showID,
		SeasonID:             seasonID,
		EpisodeNumbers:       episodeNumbers,
		ShouldGetNextEpisode: shouldGetNextEpisode,
	}, token, nil)
}

func (c *Client) RemoveEpisodeLog(token string, showID, seasonID int, episodeNumbers []int) error {
	return c.post("/episode_log/remove", episodeLogBody{
		ShowID:         showID,
		SeasonID:       seasonID,
		EpisodeNumbers: episodeNumbers,
	}, token, nil)
}
