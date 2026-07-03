package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/alireza/tvtime2serializd/internal/lookupdebug"
	"github.com/alireza/tvtime2serializd/internal/outbound"
)

const defaultBaseURL = "https://api.themoviedb.org/3"

var tmdbRetryDelays = []time.Duration{
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

// Client queries the TMDB API for external ID mappings.
type Client struct {
	http    *http.Client
	apiKey  string
	baseURL string
	gate    *outbound.Gate

	mu    sync.Mutex
	cache map[int64]*int
}

func NewClient(apiKey string) *Client {
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second, Transport: transport},
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		cache:   make(map[int64]*int),
	}
}

func (c *Client) SetOutboundGate(g *outbound.Gate) {
	c.gate = g
}

type findResponse struct {
	TVResults []struct {
		ID int `json:"id"`
	} `json:"tv_results"`
}

// TMDBIDByTVDB looks up a TMDB TV series ID via TheTVDB series ID.
func (c *Client) TMDBIDByTVDB(ctx context.Context, tvdbID int64) (*int, error) {
	if c.apiKey == "" {
		lookupdebug.Printf("tmdb api skipped tvdb=%d: TMDB_API_KEY not set", tvdbID)
		return nil, nil
	}
	if cached, ok := c.cached(tvdbID); ok {
		return cached, nil
	}

	lookupdebug.Printf("tmdb api find tvdb=%d", tvdbID)
	url := fmt.Sprintf("%s/find/%d?external_source=tvdb_id&language=en-US", c.baseURL, tvdbID)

	body, status, err := c.getWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("tmdb find failed (%d): %s", status, body)
	}

	var parsed findResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode tmdb find response: %w", err)
	}
	if len(parsed.TVResults) == 0 {
		c.store(tvdbID, nil)
		return nil, nil
	}

	tmdbID := parsed.TVResults[0].ID
	c.store(tvdbID, &tmdbID)
	return &tmdbID, nil
}

func (c *Client) getWithRetry(ctx context.Context, url string) ([]byte, int, error) {
	var lastErr error
	var lastStatus int
	var lastBody []byte

	for attempt := 0; attempt <= len(tmdbRetryDelays); attempt++ {
		if attempt > 0 {
			delay := tmdbRetryDelays[attempt-1]
			lookupdebug.Printf("tmdb api retry %d/%d after %s", attempt, len(tmdbRetryDelays), delay)
			if err := sleepContext(ctx, delay); err != nil {
				return nil, 0, err
			}
		}

		body, status, err := c.doGet(ctx, url)
		if err != nil {
			lastErr = err
			if attempt < len(tmdbRetryDelays) {
				continue
			}
			return nil, 0, err
		}
		if status < 400 {
			return body, status, nil
		}

		lastStatus = status
		lastBody = body
		lastErr = fmt.Errorf("tmdb find failed (%d): %s", status, body)
		if !isRetryableStatus(status) || attempt == len(tmdbRetryDelays) {
			return lastBody, lastStatus, lastErr
		}
	}
	return lastBody, lastStatus, lastErr
}

func (c *Client) doGet(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	var body []byte
	var status int
	err = c.gate.Call(ctx, func() (int, error) {
		res, doErr := c.http.Do(req)
		if doErr != nil {
			return 0, doErr
		}
		defer res.Body.Close()

		raw, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return res.StatusCode, readErr
		}
		body = raw
		status = res.StatusCode
		return status, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return body, status, nil
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) cached(tvdbID int64) (*int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.cache[tvdbID]
	return v, ok
}

func (c *Client) store(tvdbID int64, tmdbID *int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[tvdbID] = tmdbID
}
