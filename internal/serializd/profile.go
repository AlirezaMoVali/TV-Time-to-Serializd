package serializd

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type ProfilePageOptions struct {
	SortBy string
}

func (c *Client) GetCurrentlyWatchingPage(username string, page int, opts ProfilePageOptions) (json.RawMessage, error) {
	return c.getProfilePage(username, "currently_watching_page", page, opts)
}

func (c *Client) GetWatchlistPage(username string, page int, opts ProfilePageOptions) (json.RawMessage, error) {
	return c.getProfilePage(username, "watchlistpage_v2", page, opts)
}

func (c *Client) GetWatchedPage(username string, page int, opts ProfilePageOptions) (json.RawMessage, error) {
	return c.getProfilePage(username, "watchedpage_v2", page, opts)
}

func (c *Client) GetPausedPage(username string, page int, opts ProfilePageOptions) (json.RawMessage, error) {
	return c.getProfilePage(username, "paused_shows_page", page, opts)
}

func (c *Client) GetDroppedPage(username string, page int, opts ProfilePageOptions) (json.RawMessage, error) {
	return c.getProfilePage(username, "dropped_shows_page", page, opts)
}

func (c *Client) GetDiary(username string) (json.RawMessage, error) {
	var data json.RawMessage
	path := fmt.Sprintf("/user/%s/diary", username)
	if err := c.get(path, nil, "", &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) getProfilePage(username, resource string, page int, opts ProfilePageOptions) (json.RawMessage, error) {
	var data json.RawMessage
	path := fmt.Sprintf("/user/%s/%s/%d", username, resource, page)

	query := url.Values{}
	if opts.SortBy != "" {
		query.Set("sort_by", opts.SortBy)
	}

	if err := c.get(path, query, "", &data); err != nil {
		return nil, err
	}
	return data, nil
}
