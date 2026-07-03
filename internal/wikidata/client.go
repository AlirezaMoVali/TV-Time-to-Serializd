package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alireza/tvtime2serializd/internal/lookupdebug"
	"github.com/alireza/tvtime2serializd/internal/outbound"
)

const defaultEndpoint = "https://query.wikidata.org/sparql"

// security: IMDB IDs are interpolated into SPARQL — restrict to tt + digits only.
var imdbIDPattern = regexp.MustCompile(`^tt\d+$`)

var wikidataRetryDelays = []time.Duration{
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

// Client queries Wikidata for external ID mappings.
type Client struct {
	http     *http.Client
	endpoint string
	gate     *outbound.Gate
	mu       sync.Mutex
	tvdbCache map[int64]*int
	imdbCache map[string]*int
}

func NewClient() *Client {
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		http:      &http.Client{Timeout: 15 * time.Second, Transport: transport},
		endpoint:  defaultEndpoint,
		tvdbCache: make(map[int64]*int),
		imdbCache: make(map[string]*int),
	}
}

func (c *Client) SetOutboundGate(g *outbound.Gate) {
	c.gate = g
}

type sparqlResponse struct {
	Results struct {
		Bindings []map[string]sparqlValue `json:"bindings"`
	} `json:"results"`
}

type sparqlValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TMDBIDByTVDB looks up a TMDB TV series ID (P4983) via TheTVDB series ID (P4835).
func (c *Client) TMDBIDByTVDB(ctx context.Context, tvdbID int64) (*int, error) {
	if cached, ok := c.cachedTVDB(tvdbID); ok {
		return cached, nil
	}

	query := fmt.Sprintf(`SELECT ?show ?showLabel ?tmdb WHERE {
  ?show wdt:P4835 "%s".
  ?show wdt:P4983 ?tmdb.
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}`, strconv.FormatInt(tvdbID, 10))

	tmdbID, err := c.queryTMDB(ctx, query)
	if err != nil {
		return nil, err
	}
	c.storeTVDBCache(tvdbID, tmdbID)
	return tmdbID, nil
}

// TMDBIDByIMDB looks up a TMDB TV series ID (P4983) via IMDb ID (P345).
func (c *Client) TMDBIDByIMDB(ctx context.Context, imdbID string) (*int, error) {
	normalized := normalizeIMDBID(imdbID)
	if normalized == "" {
		return nil, nil
	}
	if cached, ok := c.cachedIMDB(normalized); ok {
		return cached, nil
	}

	query := fmt.Sprintf(`SELECT ?show ?tmdb WHERE {
  ?show wdt:P345 "%s".
  ?show wdt:P4983 ?tmdb.
}`, normalized)

	tmdbID, err := c.queryTMDB(ctx, query)
	if err != nil {
		return nil, err
	}
	c.storeIMDBCache(normalized, tmdbID)
	return tmdbID, nil
}

func (c *Client) queryTMDB(ctx context.Context, sparql string) (*int, error) {
	body, err := c.query(ctx, sparql)
	if err != nil {
		return nil, err
	}

	var resp sparqlResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode sparql response: %w", err)
	}
	if len(resp.Results.Bindings) == 0 {
		return nil, nil
	}

	raw := resp.Results.Bindings[0]["tmdb"].Value
	tmdbID, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("parse tmdb id %q: %w", raw, err)
	}
	return &tmdbID, nil
}

func (c *Client) query(ctx context.Context, sparql string) ([]byte, error) {
	form := url.Values{"query": {sparql}}

	var lastErr error
	for attempt := 0; attempt <= len(wikidataRetryDelays); attempt++ {
		if attempt > 0 {
			delay := wikidataRetryDelays[attempt-1]
			lookupdebug.Printf("wikidata query retry %d/%d after %s", attempt, len(wikidataRetryDelays), delay)
			if err := sleepContext(ctx, delay); err != nil {
				return nil, err
			}
		}

		body, status, err := c.doQuery(ctx, form)
		if err != nil {
			lastErr = err
			if attempt < len(wikidataRetryDelays) {
				continue
			}
			return nil, err
		}
		if status < 400 {
			return body, nil
		}
		lastErr = fmt.Errorf("wikidata query failed (%d): %s", status, body)
		if !isRetryableStatus(status) || attempt == len(wikidataRetryDelays) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func (c *Client) doQuery(ctx context.Context, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+json")
	req.Header.Set("User-Agent", "tvtime2serializd/1.0 (wikidata tmdb lookup)")

	var raw []byte
	var status int
	err = c.gate.Call(ctx, func() (int, error) {
		res, doErr := c.http.Do(req)
		if doErr != nil {
			return 0, doErr
		}
		defer res.Body.Close()

		body, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return res.StatusCode, readErr
		}
		raw = body
		status = res.StatusCode
		return status, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return raw, status, nil
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

func normalizeIMDBID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(id), "tt") {
		id = "tt" + id
	}
	id = strings.ToLower(id)
	if !imdbIDPattern.MatchString(id) {
		return ""
	}
	return id
}

func (c *Client) cachedTVDB(tvdbID int64) (*int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.tvdbCache[tvdbID]
	return v, ok
}

func (c *Client) cachedIMDB(imdbID string) (*int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.imdbCache[imdbID]
	return v, ok
}

func (c *Client) storeTVDBCache(tvdbID int64, tmdbID *int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tvdbCache[tvdbID] = tmdbID
}

func (c *Client) storeIMDBCache(imdbID string, tmdbID *int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.imdbCache[imdbID] = tmdbID
}
