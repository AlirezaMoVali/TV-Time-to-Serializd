package serializd

import "fmt"

func (c *Client) Login(email, password string) (string, error) {
	var resp LoginResponse
	err := c.post("/login", map[string]string{
		"email":    email,
		"password": password,
	}, "", &resp)
	if err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", fmt.Errorf("login response missing token")
	}
	return resp.Token, nil
}

func (c *Client) ValidateAuthToken(token string) error {
	return c.post("/validateauthtoken", map[string]string{
		"token": token,
	}, "", nil)
}
