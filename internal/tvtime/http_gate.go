package tvtime

import (
	"context"
	"net/http"

	"github.com/alireza/tvtime2serializd/internal/outbound"
)

func (c *Client) SetOutboundGate(g *outbound.Gate) {
	c.gate = g
}

func (c *Client) roundTrip(ctx context.Context, httpClient *http.Client, req *http.Request) (*http.Response, error) {
	if httpClient == nil {
		httpClient = c.http
	}
	var resp *http.Response
	err := c.gate.Call(ctx, func() (int, error) {
		var err error
		resp, err = httpClient.Do(req)
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
