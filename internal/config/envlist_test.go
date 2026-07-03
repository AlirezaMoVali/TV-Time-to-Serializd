package config

import (
	"testing"
)

func TestSplitEnvList(t *testing.T) {
	t.Setenv("TEST_ENV_LIST", " https://a.example.com , https://b.example.com , , ")

	got := splitEnvList("TEST_ENV_LIST")
	want := []string{"https://a.example.com", "https://b.example.com"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitEnvListEmpty(t *testing.T) {
	t.Parallel()

	if got := splitEnvList("TEST_ENV_LIST_MISSING"); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
