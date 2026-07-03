package tvtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alireza/tvtime2serializd/internal/applog"
)

const (
	maxPageRetries          = 5
	maxConsecutivePageFails = 5
)

func (c *Client) paginatedHTTP() *http.Client {
	return c.http
}

type pageProgressFunc func(fetched, page int)

func (c *Client) fetchObjects(token, innerURL, entityType string, pageLimit int, onPage ...pageProgressFunc) ([]map[string]any, error) {
	var progress pageProgressFunc
	if len(onPage) > 0 {
		progress = onPage[0]
	}

	oB64 := b64url(innerURL)
	base := fmt.Sprintf("%s?o_b64=%s&entity_type=%s&page_limit=%d", c.apiSidecarBase(), oB64, entityType, pageLimit)

	httpClient := c.paginatedHTTP()
	var all []map[string]any
	pageOffset := 0
	var lastFirstUUID string
	consecutiveFails := 0
	pagesFetched := 0

pageLoop:
	for {
		reqURL := base + "&page_offset=" + strconv.Itoa(pageOffset)

		body, status, err := c.fetchSidecarPage(httpClient, reqURL, token)
		if err != nil {
			if status >= 400 && status < 500 {
				applog.Warn("tvtime fetch stopped",
					"entity", entityType,
					"offset", pageOffset,
					"err", err,
				)
				break pageLoop
			}
			applog.Warn("tvtime fetch page failed",
				"entity", entityType,
				"offset", pageOffset,
				"err", err,
			)
			consecutiveFails++
			if consecutiveFails >= maxConsecutivePageFails {
				applog.Warn("tvtime fetch aborted",
					"entity", entityType,
					"consecutive_failures", consecutiveFails,
				)
				break pageLoop
			}
			pageOffset += pageLimit
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			applog.Warn("tvtime fetch decode failed",
				"entity", entityType,
				"offset", pageOffset,
				"err", err,
			)
			consecutiveFails++
			if consecutiveFails >= maxConsecutivePageFails {
				break pageLoop
			}
			pageOffset += pageLimit
			continue
		}

		objects := extractObjects(raw)
		if len(objects) == 0 {
			break
		}

		firstUUID := ""
		if len(objects) > 0 {
			firstUUID, _ = objects[0]["uuid"].(string)
		}
		if firstUUID != "" && firstUUID == lastFirstUUID {
			applog.Info("tvtime fetch duplicate page",
				"entity", entityType,
				"offset", pageOffset,
			)
			break
		}

		all = append(all, objects...)
		lastFirstUUID = firstUUID
		consecutiveFails = 0
		pagesFetched++

		if progress != nil {
			progress(len(all), pagesFetched)
		}

		if len(objects) < pageLimit {
			break
		}
		pageOffset += pageLimit
	}

	applog.Info("tvtime fetch complete",
		"entity", entityType,
		"count", len(all),
		"pages", pagesFetched,
	)
	return all, nil
}

func (c *Client) fetchSidecarPage(httpClient *http.Client, reqURL, token string) ([]byte, int, error) {
	var lastStatus int
	var lastErr error

	for attempt := 0; attempt <= maxPageRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			time.Sleep(delay)
		}

		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, 0, err
		}
		setSidecarHeaders(req, token)

		resp, err := c.roundTrip(context.Background(), httpClient, req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		lastStatus = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, resp.StatusCode, nil
		}

		lastErr = fmt.Errorf("sidecar GET failed (%d): %s", resp.StatusCode, body)
		if resp.StatusCode == http.StatusTooManyRequests {
			continue
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return body, resp.StatusCode, lastErr
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("sidecar GET failed (%d)", lastStatus)
	}
	return nil, lastStatus, lastErr
}

func (c *Client) fetchObjectsWithRetry(token, innerURL, entityType string, pageLimit int) ([]map[string]any, error) {
	const maxEmptyRetries = 3
	results, err := c.fetchObjects(token, innerURL, entityType, pageLimit)
	if err != nil {
		return nil, err
	}
	for attempt := 1; attempt < maxEmptyRetries && len(results) == 0; attempt++ {
		time.Sleep(2 * time.Second)
		results, err = c.fetchObjects(token, innerURL, entityType, pageLimit)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func watchMapKey(v any) string {
	if v == nil {
		return ""
	}
	switch n := v.(type) {
	case string:
		return strings.TrimSpace(n)
	case float64:
		return strconv.FormatInt(int64(n), 10)
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
		return n.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
