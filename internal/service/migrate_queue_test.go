package service

import (
	"testing"

	"github.com/alireza/tvtime2serializd/internal/repository"
)

func TestEstimateFullMigrationSeconds(t *testing.T) {
	t.Parallel()

	full := estimateFullMigrationSeconds(false, 100)
	withDump := estimateFullMigrationSeconds(true, 100)
	if withDump <= full {
		t.Fatalf("dump should increase estimate: full=%v dump=%v", full, withDump)
	}
}

func TestEstimateRemainingSecondsGatherPhase(t *testing.T) {
	t.Parallel()

	p := MigrateProgress{
		Status:         string(repository.MigrateRunning),
		TVTimeLogin:    LoginStep{Status: StepDone},
		SerializdLogin: LoginStep{Status: StepDone},
		GatherShows: CountStep{
			Status: StepRunning,
			Total:  200,
			Done:   50,
		},
	}

	early := estimateRemainingSeconds(p, false)
	p.GatherShows.Done = 150
	late := estimateRemainingSeconds(p, false)
	if late >= early {
		t.Fatalf("more gather progress should reduce ETA: early=%v late=%v", early, late)
	}
}

func TestEstimateRemainingSecondsCompleted(t *testing.T) {
	t.Parallel()

	p := MigrateProgress{Status: string(repository.MigrateCompleted)}
	if sec := estimateRemainingSeconds(p, false); sec != 0 {
		t.Fatalf("completed job should have 0 remaining, got %v", sec)
	}
}

func TestEstimateQueuedJobSecondsSkipsVerifiedLogin(t *testing.T) {
	t.Parallel()

	full := estimateFullMigrationSeconds(false, 100)
	verified := estimateQueuedJobSeconds(false, 100, MigrateProgress{
		TVTimeLogin:    LoginStep{Status: StepDone},
		SerializdLogin: LoginStep{Status: StepDone},
	})
	if verified >= full {
		t.Fatalf("verified queue estimate should be less than full: full=%v verified=%v", full, verified)
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	if got := formatDuration(45); got != "45s" {
		t.Fatalf("got %q", got)
	}
	if got := formatDuration(125); got != "2m 5s" {
		t.Fatalf("got %q", got)
	}
}
