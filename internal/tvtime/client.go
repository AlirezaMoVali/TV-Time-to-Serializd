package tvtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alireza/tvtime2serializd/internal/outbound"
	"github.com/alireza/tvtime2serializd/internal/safenum"
)

const (
	createUserURL = "https://api2.tozelabs.com/v2/user?lang=en&timezone=UTC&country_code=us&source=web&version=10.10.0"
	authLoginURL  = "https://auth.tvtime.com/v1/login"
	sidecarBase   = "https://app.tvtime.com/sidecar"
)

type Client struct {
	http        *http.Client
	sidecarBase string
	gate        *outbound.Gate
}

func NewClient() *Client {
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		http: &http.Client{Timeout: 90 * time.Second, Transport: transport},
	}
}

func (c *Client) apiSidecarBase() string {
	if c.sidecarBase != "" {
		return c.sidecarBase
	}
	return sidecarBase
}

type Tokens struct {
	UserID          int64  `json:"user_id"`
	JWTToken        string `json:"jwt_token"`
	JWTRefreshToken string `json:"jwt_refresh_token"`
}

func (c *Client) Login(email, password string) (*Tokens, error) {
	anon, err := c.createAnonymousSession()
	if err != nil {
		return nil, fmt.Errorf("anonymous session: %w", err)
	}

	anonJWT, ok := anon["jwt_token"].(string)
	if !ok || anonJWT == "" {
		return nil, fmt.Errorf("anonymous session missing jwt_token")
	}

	data, err := c.login(email, password, anonJWT)
	if err != nil {
		return nil, err
	}

	tokens := &Tokens{
		JWTToken:        asString(data["jwt_token"]),
		JWTRefreshToken: asString(data["jwt_refresh_token"]),
	}
	if id, ok := data["id"].(float64); ok {
		if userID, ok := safenum.Float64ToInt64(id); ok {
			tokens.UserID = userID
		}
	}

	return tokens, nil
}

func (c *Client) createAnonymousSession() (map[string]any, error) {
	return c.sidecar(createUserURL, http.MethodPost, nil, "")
}

func (c *Client) login(email, password, bearer string) (map[string]any, error) {
	body := map[string]string{
		"username": email,
		"password": password,
	}
	result, err := c.sidecar(authLoginURL, http.MethodPost, body, bearer)
	if err != nil {
		return nil, err
	}

	status, _ := result["status"].(string)
	if status != "success" {
		return nil, fmt.Errorf("login failed: %v", result)
	}

	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("login response missing data")
	}
	return data, nil
}

func (c *Client) sidecar(targetURL, method string, body any, bearer string) (map[string]any, error) {
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.apiSidecarBase()+"?o_b64="+b64url(targetURL), payload)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://app.tvtime.com")
	req.Header.Set("Referer", "https://app.tvtime.com/email")
	req.Header.Set("app-version", "2025082201")
	req.Header.Set("client-version", "10.10.0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tvtime2serializd/1.0)")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := c.roundTrip(context.Background(), c.http, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s failed (%d): %s", method, targetURL, resp.StatusCode, raw)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func b64url(s string) string {
	return strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(s)), "=")
}
