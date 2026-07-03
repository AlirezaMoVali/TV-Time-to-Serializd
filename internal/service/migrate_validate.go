package service

import (
	"context"
	"fmt"
)

// CredentialValidationError is returned when migrate init credentials fail verification.
type CredentialValidationError struct {
	Service string
	Cause   error
}

func (e *CredentialValidationError) Error() string {
	return fmt.Sprintf("%s login failed: %s", e.Service, e.Cause.Error())
}

func (e *CredentialValidationError) Unwrap() error {
	return e.Cause
}

func (s *MigrateService) validateCredentials(_ context.Context, req MigrateInitRequest) error {
	if _, err := s.tvtime.Login(req.TVTimeEmail, req.TVTimePassword); err != nil {
		return &CredentialValidationError{Service: "TV Time", Cause: err}
	}
	if _, err := s.serializd.Login(req.SerializdEmail, req.SerializdPassword); err != nil {
		return &CredentialValidationError{Service: "Serializd", Cause: err}
	}
	return nil
}
