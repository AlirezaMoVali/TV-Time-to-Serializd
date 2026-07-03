package account

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Hash returns a deterministic one-way SHA-256 hex digest of a normalized email.
// Plaintext email must not be stored; use this digest as the account identifier.
func Hash(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
