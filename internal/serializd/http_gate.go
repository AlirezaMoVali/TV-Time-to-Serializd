package serializd

import (
	"context"
	"net/http"
)

func (c *Client) roundTrip(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := c.gate.Call(ctx, func() (int, error) {
		var err error
		resp, err = c.http.Do(req)
		if err != nil {
			return 0, err
		}
		return resp.StatusCode, nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
