package cache

import (
	"strings"
	"testing"

	"github.com/alireza/tvtime2serializd/internal/account"
)

func TestImportedShowsKey_NoPlaintextEmail(t *testing.T) {
	email := "secret@example.com"
	key := importedShowsKey(email)
	if strings.Contains(key, email) {
		t.Fatalf("redis key must not contain plaintext email: %q", key)
	}
	if !strings.HasPrefix(key, "serializd:imported:") {
		t.Fatalf("unexpected key prefix: %q", key)
	}
	if !strings.HasSuffix(key, account.Hash(email)) {
		t.Fatalf("redis key suffix must be account hash")
	}
}
