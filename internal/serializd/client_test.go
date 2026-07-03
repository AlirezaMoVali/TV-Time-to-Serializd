package serializd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-requested-with") != requestedWith {
			t.Fatalf("missing x-requested-with header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"jwt-test"}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	token, err := client.Login("user@example.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if token != "jwt-test" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestSeasonSeasonIDValue(t *testing.T) {
	if got := (Season{SeasonID: 42}).SeasonIDValue(); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := (Season{ID: 7}).SeasonIDValue(); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	err := client.ValidateAuthToken("bad")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", apiErr.StatusCode)
	}
}

func TestGetShowDecodesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt" {
			t.Fatalf("missing auth header")
		}
		_, _ = w.Write([]byte(`{"id":1399,"name":"Game of Thrones","status":"Ended","seasons":[{"id":3624,"seasonId":3624,"seasonNumber":1,"name":"Season 1","episodeCount":10}]}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	show, err := client.GetShow("jwt", 1399)
	if err != nil {
		t.Fatal(err)
	}
	if show.Name != "Game of Thrones" || len(show.Seasons) != 1 {
		t.Fatalf("unexpected show: %+v", show)
	}
}

func TestGetUserInformationQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("shouldGetUserContext") != "true" {
			t.Fatalf("missing query param: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"context":{"watchlist":[]}}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	info, err := client.GetUserInformation("jwt")
	if err != nil {
		t.Fatal(err)
	}
	var ctx map[string]json.RawMessage
	if err := json.Unmarshal(info.Context, &ctx); err != nil {
		t.Fatal(err)
	}
}

func TestClientRetriesTransientAPIErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"token":"jwt-retry"}`))
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	token, err := client.Login("user@example.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if token != "jwt-retry" {
		t.Fatalf("unexpected token: %s", token)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestClientDoesNotRetryUnauthorized(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewClient()
	client.BaseURL = srv.URL + "/api"

	if err := client.ValidateAuthToken("bad"); err == nil {
		t.Fatal("expected error")
	} else if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestIsRetryableSerializdError(t *testing.T) {
	if isRetryableSerializdError(&APIError{StatusCode: 502}) != true {
		t.Fatal("502 should retry")
	}
	if isRetryableSerializdError(&APIError{StatusCode: 401}) != false {
		t.Fatal("401 should not retry")
	}
	if isRetryableSerializdError(fmt.Errorf("Get %q: EOF", "https://serializd.onrender.com")) != true {
		t.Fatal("network EOF should retry")
	}
}
