package serializd

type showIDBody struct {
	ShowID int `json:"show_id"`
}

type showSeasonsBody struct {
	ShowID    int   `json:"show_id"`
	SeasonIDs []int `json:"season_ids"`
}

type watchlistRemoveBody struct {
	ShowID    int   `json:"show_id"`
	SeasonIDs []int `json:"season_ids"`
	Async     bool  `json:"async"`
}

type watchedAddBody struct {
	ShowID    int   `json:"show_id"`
	SeasonIDs []int `json:"season_ids"`
	Async     bool  `json:"async"`
}

func (c *Client) AddCurrentlyWatching(token string, showID int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/currently_watching", showIDBody{ShowID: showID}, token, &resp)
	return &resp, err
}

func (c *Client) RemoveCurrentlyWatching(token string, showID int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/currently_watching/remove", showIDBody{ShowID: showID}, token, &resp)
	return &resp, err
}

func (c *Client) AddWatchlist(token string, showID int, seasonIDs []int) error {
	return c.post("/watchlist_v2", showSeasonsBody{
		ShowID:    showID,
		SeasonIDs: seasonIDs,
	}, token, nil)
}

func (c *Client) RemoveWatchlist(token string, showID int, seasonIDs []int, async bool) error {
	return c.post("/watchlist/remove_v2", watchlistRemoveBody{
		ShowID:    showID,
		SeasonIDs: seasonIDs,
		Async:     async,
	}, token, nil)
}

func (c *Client) AddWatched(token string, showID int, seasonIDs []int, async bool) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/watched_v2", watchedAddBody{
		ShowID:    showID,
		SeasonIDs: seasonIDs,
		Async:     async,
	}, token, &resp)
	return &resp, err
}

func (c *Client) RemoveWatched(token string, showID int, seasonIDs []int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/watched/remove_v2", showSeasonsBody{
		ShowID:    showID,
		SeasonIDs: seasonIDs,
	}, token, &resp)
	return &resp, err
}

func (c *Client) AddPaused(token string, showID int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/paused_shows", showIDBody{ShowID: showID}, token, &resp)
	return &resp, err
}

func (c *Client) RemovePaused(token string, showID int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/paused_shows/remove", showIDBody{ShowID: showID}, token, &resp)
	return &resp, err
}

func (c *Client) AddDropped(token string, showID int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/dropped_shows", showIDBody{ShowID: showID}, token, &resp)
	return &resp, err
}

func (c *Client) RemoveDropped(token string, showID int) (*MessageResponse, error) {
	var resp MessageResponse
	err := c.post("/dropped_shows/remove", showIDBody{ShowID: showID}, token, &resp)
	return &resp, err
}
