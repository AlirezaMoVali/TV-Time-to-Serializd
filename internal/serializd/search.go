package serializd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ShowSearchResult struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	FirstAirDate string `json:"firstAirDate"`
}

type ShowSearchResponse struct {
	Results    []ShowSearchResult `json:"results"`
	TotalPages int                `json:"totalPages"`
}

// SearchShows queries Serializd for shows matching search_query.
func (c *Client) SearchShows(query string) (*ShowSearchResponse, error) {
	var resp ShowSearchResponse
	err := c.get("/search/shows", url.Values{"search_query": {query}}, "", &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// TMDBIDByTitleYear resolves a TMDB show ID via Serializd search using "title (year)".
func (c *Client) TMDBIDByTitleYear(title string, year *int) (*int, error) {
	query := buildShowSearchQuery(title, year)
	if query == "" {
		return nil, nil
	}

	if cached, ok := c.cachedSearch(query); ok {
		return cached, nil
	}

	resp, err := c.SearchShows(query)
	if err != nil {
		return nil, err
	}

	tmdbID := pickShowSearchResult(resp.Results, title, year)
	c.storeSearchCache(query, tmdbID)
	return tmdbID, nil
}

func buildShowSearchQuery(title string, year *int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	if year != nil && *year > 0 {
		return fmt.Sprintf("%s (%d)", title, *year)
	}
	return title
}

func pickShowSearchResult(results []ShowSearchResult, title string, year *int) *int {
	if len(results) == 0 {
		return nil
	}

	normTitle := normalizeSearchTitle(title)
	var titleOnlyMatch *int

	for _, result := range results {
		if normalizeSearchTitle(result.Name) != normTitle {
			continue
		}
		if year == nil || firstAirYear(result.FirstAirDate) == *year {
			id := result.ID
			return &id
		}
		if titleOnlyMatch == nil {
			id := result.ID
			titleOnlyMatch = &id
		}
	}

	return titleOnlyMatch
}

func normalizeSearchTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

func firstAirYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

func (c *Client) cachedSearch(query string) (*int, bool) {
	c.muSearch.Lock()
	defer c.muSearch.Unlock()
	v, ok := c.searchCache[query]
	return v, ok
}

func (c *Client) storeSearchCache(query string, tmdbID *int) {
	c.muSearch.Lock()
	defer c.muSearch.Unlock()
	c.searchCache[query] = tmdbID
}
