package account

import "testing"

func TestHash_Deterministic(t *testing.T) {
	a := Hash("User@Example.com")
	b := Hash("  user@example.com  ")
	if a != b {
		t.Fatalf("expected same hash for normalized email, got %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(a))
	}
}

func TestHash_DifferentEmails(t *testing.T) {
	if Hash("a@example.com") == Hash("b@example.com") {
		t.Fatal("expected different hashes for different emails")
	}
}
