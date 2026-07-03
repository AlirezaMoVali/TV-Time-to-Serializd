package tvtime

import (
	"fmt"
	"net/http"
)

// RefreshJWT attempts to exchange a refresh token for a new access token.
// It returns the original token when refresh is unavailable or fails.
func (c *Client) RefreshJWT(tokens *Tokens) string {
	if tokens == nil {
		return ""
	}
	if tokens.JWTRefreshToken == "" {
		return tokens.JWTToken
	}

	anon, err := c.createAnonymousSession()
	if err != nil {
		return tokens.JWTToken
	}
	anonJWT := asString(anon["jwt_token"])
	if anonJWT == "" {
		return tokens.JWTToken
	}

	candidates := []struct {
		url  string
		body any
	}{
		{
			url: "https://auth.tvtime.com/v1/refresh",
			body: map[string]string{
				"refresh_token": tokens.JWTRefreshToken,
			},
		},
		{
			url: "https://auth.tvtime.com/v1/token/refresh",
			body: map[string]string{
				"jwt_refresh_token": tokens.JWTRefreshToken,
			},
		},
		{
			url: "https://api2.tozelabs.com/v2/user/refresh",
			body: map[string]string{
				"jwt_refresh_token": tokens.JWTRefreshToken,
			},
		},
	}

	for _, candidate := range candidates {
		result, err := c.sidecar(candidate.url, http.MethodPost, candidate.body, anonJWT)
		if err != nil {
			continue
		}
		if jwt := extractJWT(result); jwt != "" {
			return jwt
		}
	}

	return tokens.JWTToken
}

func extractJWT(result map[string]any) string {
	if jwt := asString(result["jwt_token"]); jwt != "" {
		return jwt
	}
	if data, ok := result["data"].(map[string]any); ok {
		if jwt := asString(data["jwt_token"]); jwt != "" {
			return jwt
		}
	}
	return ""
}

func (c *Client) activeJWT(tokens *Tokens) string {
	if tokens == nil {
		return ""
	}
	refreshed := c.RefreshJWT(tokens)
	if refreshed != "" {
		return refreshed
	}
	return tokens.JWTToken
}

func (c *Client) userID(tokens *Tokens) int64 {
	if tokens == nil {
		return 0
	}
	return tokens.UserID
}

// ValidateTokens returns an error when export credentials are incomplete.
func ValidateTokens(tokens *Tokens) error {
	if tokens == nil || tokens.JWTToken == "" || tokens.UserID == 0 {
		return fmt.Errorf("tvtime tokens are incomplete")
	}
	return nil
}
