package config

import "testing"

func TestValidateRequiresEncryptionKey(t *testing.T) {
	t.Parallel()

	cfg := Config{
		DatabaseURL: "postgres://localhost/db",
		RedisURL:    "redis://localhost:6379/0",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing TOKEN_ENCRYPTION_KEY")
	}
}

func TestValidateOK(t *testing.T) {
	t.Parallel()

	cfg := Config{
		DatabaseURL:        "postgres://localhost/db",
		RedisURL:           "redis://localhost:6379/0",
		TokenEncryptionKey: "test",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
