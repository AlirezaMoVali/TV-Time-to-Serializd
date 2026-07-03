package serializd

import "net/url"

func (c *Client) GetUserInformation(token string) (*UserInformation, error) {
	query := url.Values{"shouldGetUserContext": {"true"}}
	var info UserInformation
	if err := c.get("/user_information", query, token, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
