package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	TokenEncryptionKey string
	TMDBAPIKey         string
	CORSAllowedOrigins []string
}

func Load() Config {
	return Config{
		Port:               envOr("PORT", "8080"),
		DatabaseURL:        envOr("DATABASE_URL", "postgres://tvtime:tvtime@localhost:5432/tvtime2serializd?sslmode=disable"),
		RedisURL:           envOr("REDIS_URL", "redis://localhost:6379/0"),
		TokenEncryptionKey: envOr("TOKEN_ENCRYPTION_KEY", ""),
		TMDBAPIKey:         envOr("TMDB_API_KEY", ""),
		CORSAllowedOrigins: splitEnvList("CORS_ALLOWED_ORIGINS"),
	}
}

// Validate checks required security-sensitive configuration at startup.
func (c Config) Validate() error {
	if c.TokenEncryptionKey == "" {
		return fmt.Errorf("TOKEN_ENCRYPTION_KEY is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	return nil
}

func (c Config) Addr() string {
	if c.Port == "" {
		return ":8080"
	}
	if c.Port[0] == ':' {
		return c.Port
	}
	return fmt.Sprintf(":%s", c.Port)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
