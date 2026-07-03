package service

import (
	"errors"
	"testing"
)

func TestCredentialValidationError(t *testing.T) {
	t.Parallel()

	err := &CredentialValidationError{Service: "TV Time", Cause: errors.New("login failed")}
	if err.Error() != "TV Time login failed: login failed" {
		t.Fatalf("unexpected message: %q", err.Error())
	}
	var credErr *CredentialValidationError
	if !errors.As(err, &credErr) {
		t.Fatal("expected errors.As to match CredentialValidationError")
	}
}
