package serializd

import (
	"encoding/json"
	"fmt"
)

func (c *Client) GetShow(token string, showID int) (*Show, error) {
	var show Show
	path := fmt.Sprintf("/show/%d", showID)
	if err := c.get(path, nil, token, &show); err != nil {
		return nil, err
	}
	return &show, nil
}

func (c *Client) GetSeason(token string, showID, seasonNumber int) (json.RawMessage, error) {
	var season json.RawMessage
	path := fmt.Sprintf("/show/%d/season/%d", showID, seasonNumber)
	if err := c.get(path, nil, token, &season); err != nil {
		return nil, err
	}
	return season, nil
}
