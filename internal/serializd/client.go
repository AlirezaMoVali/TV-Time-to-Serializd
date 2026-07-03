package serializd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/outbound"
)

const (
	APIBase            = "https://serializd.onrender.com/api"
	requestedWith      = "serializd_vercel"
	maxRequestRetries  = 5
	maxRetryBackoff    = 30 * time.Second
)

type Client struct {
	http    *http.Client
	BaseURL string
	gate    *outbound.Gate

	muSearch    sync.Mutex
	searchCache map[string]*int
}

func NewClient() *Client {
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		http:        &http.Client{Timeout: 60 * time.Second, Transport: transport},
		BaseURL:     APIBase,
		searchCache: make(map[string]*int),
	}
}

func (c *Client) SetOutboundGate(g *outbound.Gate) {
	c.gate = g
}

func (c *Client) apiBase() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return APIBase
}

func (c *Client) do(method, path string, query url.Values, body any, token string, dest any) error {
	var lastErr error
	for attempt := 0; attempt <= maxRequestRetries; attempt++ {
		if attempt > 0 {
			delay := retryBackoff(attempt)
			applog.Warn("serializd request retry",
				"method", method,
				"path", path,
				"attempt", attempt,
				"max_attempts", maxRequestRetries,
				"delay", delay.String(),
				"err", lastErr,
			)
			time.Sleep(delay)
		}
		lastErr = c.doOnce(method, path, query, body, token, dest)
		if lastErr == nil {
			return nil
		}
		if !isRetryableSerializdError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func retryBackoff(attempt int) time.Duration {
	delay := time.Duration(1<<uint(attempt-1)) * time.Second
	if delay > maxRetryBackoff {
		return maxRetryBackoff
	}
	return delay
}

func isRetryableSerializdError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusUnauthorized ||
			apiErr.StatusCode == http.StatusForbidden ||
			apiErr.StatusCode == http.StatusNotFound ||
			apiErr.StatusCode == http.StatusBadRequest {
			return false
		}
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
	}
	if strings.Contains(err.Error(), "decode response") || strings.Contains(err.Error(), "marshal body") {
		return false
	}
	return true
}

func (c *Client) doOnce(method, path string, query url.Values, body any, token string, dest any) error {
	u := c.apiBase() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		payload = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, payload)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-requested-with", requestedWith)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tvtime2serializd/1.0)")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.roundTrip(context.Background(), req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}

	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) get(path string, query url.Values, token string, dest any) error {
	return c.do(http.MethodGet, path, query, nil, token, dest)
}

func (c *Client) post(path string, body any, token string, dest any) error {
	return c.do(http.MethodPost, path, nil, body, token, dest)
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("serializd api error (%d): %s", e.StatusCode, e.Body)
}
