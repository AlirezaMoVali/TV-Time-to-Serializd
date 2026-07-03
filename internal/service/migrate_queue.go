package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/google/uuid"
)

const (
	defaultQueueConcurrency = 4
	maxQueueConcurrency     = 5
	defaultAssumedShows     = 150

	etaLoginTVTimeSec     = 8.0
	etaLoginSerializdSec  = 22.0
	etaGatherBaseSec      = 25.0
	etaGatherPerShowSec   = 1.4
	etaCheckPerShowSec    = 0.45
	etaImportPerShowSec   = 2.8
	etaImportPendingRatio = 0.55
	etaDumpPerShowSec     = 0.12
)

type queueItem struct {
	jobID      uuid.UUID
	req        MigrateInitRequest
	enqueuedAt time.Time
	startedAt  time.Time
}

type migrateQueue struct {
	mu      sync.Mutex
	max     int
	pending []*queueItem
	running map[uuid.UUID]*queueItem
	byID    map[uuid.UUID]*queueItem
}

func newMigrateQueue() *migrateQueue {
	return &migrateQueue{
		max:     queueConcurrency(),
		pending: []*queueItem{},
		running: make(map[uuid.UUID]*queueItem),
		byID:    make(map[uuid.UUID]*queueItem),
	}
}

func queueConcurrency() int {
	raw := os.Getenv("MIGRATE_QUEUE_CONCURRENCY")
	if raw == "" {
		return defaultQueueConcurrency
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultQueueConcurrency
	}
	if n > maxQueueConcurrency {
		return maxQueueConcurrency
	}
	return n
}

func (s *MigrateService) enqueue(jobID uuid.UUID, req MigrateInitRequest) {
	s.queue.mu.Lock()
	item := &queueItem{jobID: jobID, req: req, enqueuedAt: time.Now()}
	s.queue.pending = append(s.queue.pending, item)
	s.queue.byID[jobID] = item
	s.queue.mu.Unlock()

	s.tryStartQueuedJobs()
}

func (s *MigrateService) releaseQueueSlot(jobID uuid.UUID) {
	s.queue.mu.Lock()
	delete(s.queue.running, jobID)
	delete(s.queue.byID, jobID)
	s.queue.mu.Unlock()

	s.tryStartQueuedJobs()
}

func (s *MigrateService) tryStartQueuedJobs() {
	var starters []*queueItem

	s.queue.mu.Lock()
	for len(s.queue.running) < s.queue.max && len(s.queue.pending) > 0 {
		item := s.queue.pending[0]
		s.queue.pending = s.queue.pending[1:]
		item.startedAt = time.Now()
		s.queue.running[item.jobID] = item
		starters = append(starters, item)
	}
	s.queue.mu.Unlock()

	for _, item := range starters {
		go s.runQueued(item.jobID, item.req)
	}
}

func (s *MigrateService) runQueued(jobID uuid.UUID, req MigrateInitRequest) {
	defer s.releaseQueueSlot(jobID)

	state := s.getState(jobID)
	if state == nil {
		applog.Error("migrate state missing", "job_id", jobID)
		return
	}

	state.progress.Status = string(repository.MigrateRunning)
	state.progress.Queue = QueueInfo{}
	state.progress.CurrentActivity = "Starting migration…"
	s.persistProgress(context.Background(), jobID, state, repository.MigrateRunning)

	s.run(jobID, req)
}

func (s *MigrateService) queueInfo(ctx context.Context, jobID uuid.UUID) QueueInfo {
	s.queue.mu.Lock()
	maxSlots := s.queue.max
	runningCount := len(s.queue.running)
	pending := append([]*queueItem(nil), s.queue.pending...)
	running := make([]*queueItem, 0, len(s.queue.running))
	for _, item := range s.queue.running {
		running = append(running, item)
	}
	s.queue.mu.Unlock()

	sortQueueItems(running)

	if findQueueItem(running, jobID) != nil {
		return QueueInfo{
			Position:         0,
			Ahead:            0,
			RunningSlotsUsed: runningCount,
			RunningSlotsMax:  maxSlots,
		}
	}

	pendingIndex := -1
	for i, item := range pending {
		if item.jobID == jobID {
			pendingIndex = i
			break
		}
	}
	if pendingIndex < 0 {
		return QueueInfo{}
	}

	ahead := runningCount + pendingIndex
	waitSec := 0.0
	for _, item := range running {
		waitSec += estimateRemainingSeconds(s.progressForJob(ctx, item.jobID), item.req.DumpEnabled)
	}
	for i := 0; i < pendingIndex; i++ {
		waitSec += estimateQueuedJobSeconds(
			pending[i].req.DumpEnabled,
			showCountHint(pending[i].jobID, s),
			s.progressForJob(ctx, pending[i].jobID),
		)
	}

	return QueueInfo{
		Position:             pendingIndex + 1,
		Ahead:                ahead,
		RunningSlotsUsed:     runningCount,
		RunningSlotsMax:      maxSlots,
		EstimatedWaitSeconds: int(waitSec + 0.5),
	}
}

func showCountHint(jobID uuid.UUID, s *MigrateService) int {
	if state := s.getState(jobID); state != nil {
		if n := showCountFromProgress(state.progress); n > 0 {
			return n
		}
	}
	return defaultAssumedShows
}

func (s *MigrateService) progressForJob(ctx context.Context, jobID uuid.UUID) MigrateProgress {
	if state := s.getState(jobID); state != nil {
		return state.progress
	}
	var progress MigrateProgress
	ok, err := s.progress.Get(ctx, jobID, &progress)
	if err != nil || !ok {
		return MigrateProgress{}
	}
	return progress
}

func sortQueueItems(items []*queueItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].startedAt.Before(items[j-1].startedAt); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func findQueueItem(items []*queueItem, jobID uuid.UUID) *queueItem {
	for _, item := range items {
		if item.jobID == jobID {
			return item
		}
	}
	return nil
}

func queueActivity(q QueueInfo) string {
	if q.Position == 0 && q.Ahead == 0 {
		return ""
	}
	if q.EstimatedWaitSeconds <= 0 {
		return fmt.Sprintf("Waiting in migration queue (position %d, %d ahead)…", q.Position, q.Ahead)
	}
	return fmt.Sprintf(
		"Waiting in migration queue (position %d, %d ahead, ~%s)…",
		q.Position,
		q.Ahead,
		formatDuration(q.EstimatedWaitSeconds),
	)
}

func formatDuration(seconds int) string {
	if seconds < 60 {
		return strconv.Itoa(seconds) + "s"
	}
	mins := seconds / 60
	if mins < 60 {
		rem := seconds % 60
		if rem == 0 {
			return strconv.Itoa(mins) + "m"
		}
		return fmt.Sprintf("%dm %ds", mins, rem)
	}
	hours := mins / 60
	mins = mins % 60
	if mins == 0 {
		return strconv.Itoa(hours) + "h"
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func estimateFullMigrationSeconds(dumpEnabled bool, shows int) float64 {
	if shows <= 0 {
		shows = defaultAssumedShows
	}
	sec := etaLoginTVTimeSec + etaLoginSerializdSec + etaGatherBaseSec + float64(shows)*etaGatherPerShowSec
	sec += float64(shows) * etaCheckPerShowSec
	sec += float64(shows) * etaImportPerShowSec * etaImportPendingRatio
	if dumpEnabled {
		sec += float64(shows) * etaDumpPerShowSec
	}
	return sec
}

func estimateRemainingSeconds(p MigrateProgress, dumpEnabled bool) float64 {
	if p.Status == string(repository.MigrateCompleted) || p.Status == string(repository.MigrateFailed) {
		return 0
	}

	shows := showCountFromProgress(p)
	if shows <= 0 {
		shows = defaultAssumedShows
	}

	var sec float64

	switch {
	case p.TVTimeLogin.Status != StepDone:
		sec += etaLoginTVTimeSec
		if p.SerializdLogin.Status != StepDone {
			sec += etaLoginSerializdSec
		}
	case p.SerializdLogin.Status != StepDone:
		sec += etaLoginSerializdSec
	}

	if p.GatherShows.Status != StepDone {
		sec += gatherRemainingSeconds(p.GatherShows, shows)
	}

	if p.CheckExisting.Status != StepDone && p.CheckExisting.Status != StepPending {
		total := p.CheckExisting.Total
		if total <= 0 {
			total = shows
		}
		remaining := p.CheckExisting.Remaining
		if remaining <= 0 && p.CheckExisting.Done < total {
			remaining = total - p.CheckExisting.Done
		}
		sec += float64(remaining) * etaCheckPerShowSec
	} else if p.CheckExisting.Status == StepPending && p.GatherShows.Status == StepDone {
		sec += float64(shows) * etaCheckPerShowSec
	}

	if p.ImportShows.Status != StepDone && p.ImportShows.Status != StepPending {
		remaining := p.ImportShows.Remaining
		if remaining <= 0 && p.ImportShows.Total > p.ImportShows.Done {
			remaining = p.ImportShows.Total - p.ImportShows.Done
		}
		sec += float64(remaining) * etaImportPerShowSec
	} else if p.ImportShows.Status == StepRunning ||
		(p.CheckExisting.Status == StepDone && p.ImportShows.Status == StepPending) {
		pending := p.ImportShows.Total
		if pending <= 0 {
			pending = int(float64(shows) * etaImportPendingRatio)
		}
		remaining := p.ImportShows.Remaining
		if remaining <= 0 {
			remaining = pending - p.ImportShows.Done
		}
		if remaining > 0 {
			sec += float64(remaining) * etaImportPerShowSec
		}
	}

	if dumpEnabled && p.Summary.Status != StepDone && p.GatherShows.Status == StepDone {
		sec += float64(shows) * etaDumpPerShowSec
	}

	if p.Summary.Status != StepDone && p.Summary.Status != StepPending && sec < 5 {
		sec = 5
	}

	return sec
}

func estimateQueuedJobSeconds(dumpEnabled bool, shows int, p MigrateProgress) float64 {
	sec := estimateFullMigrationSeconds(dumpEnabled, shows)
	if p.TVTimeLogin.Status == StepDone {
		sec -= etaLoginTVTimeSec
	}
	if p.SerializdLogin.Status == StepDone {
		sec -= etaLoginSerializdSec
	}
	if sec < 0 {
		return 0
	}
	return sec
}

func gatherRemainingSeconds(g CountStep, shows int) float64 {
	if g.Status == StepDone {
		return 0
	}
	total := g.Total
	if total <= 0 {
		total = shows
	}
	if total <= 0 {
		return etaGatherBaseSec + float64(defaultAssumedShows)*etaGatherPerShowSec
	}
	remaining := total - g.Done
	if remaining < 0 {
		remaining = 0
	}
	return etaGatherBaseSec*0.3 + float64(remaining)*etaGatherPerShowSec
}

func showCountFromProgress(p MigrateProgress) int {
	if p.Summary.TotalTVTime > 0 {
		return p.Summary.TotalTVTime
	}
	if p.GatherShows.Total > 0 {
		return p.GatherShows.Total
	}
	if p.GatherShows.Done > 0 {
		return p.GatherShows.Done
	}
	if p.CheckExisting.Total > 0 {
		return p.CheckExisting.Total
	}
	return 0
}

func (s *MigrateService) enrichQueuedProgress(ctx context.Context, progress *MigrateProgress, jobID uuid.UUID) {
	if progress.Status != "queued" {
		return
	}
	progress.Queue = s.queueInfo(ctx, jobID)
	if activity := queueActivity(progress.Queue); activity != "" {
		progress.CurrentActivity = activity
	}
}
