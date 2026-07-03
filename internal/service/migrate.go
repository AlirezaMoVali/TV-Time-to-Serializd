package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/serializd"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
)

type MigrateInitRequest struct {
	TVTimeEmail       string
	TVTimePassword    string
	SerializdEmail    string
	SerializdPassword string
	DumpEnabled       bool
	DumpFormat        tvtime.OutputFormat
}

// MigrateDeps groups dependencies for MigrateService construction.
type MigrateDeps struct {
	TVTime          *tvtime.Client
	Serializd       *serializd.Client
	Tokens          *repository.TokenRepository
	ShowLookup      *ShowLookupService
	Exports         *repository.ExportRepository
	Jobs            *repository.MigrateJobRepository
	Progress        *cache.MigrateProgressCache
	ImportedShows   *cache.ImportedShowsCache
	ImportedShowsDB *repository.SerializdImportedRepository
}

type MigrateService struct {
	tvtime          *tvtime.Client
	serializd       *serializd.Client
	tokens          *repository.TokenRepository
	showLookup      *ShowLookupService
	exports         *repository.ExportRepository
	jobs            *repository.MigrateJobRepository
	progress        *cache.MigrateProgressCache
	importedShows   *cache.ImportedShowsCache
	importedShowsDB *repository.SerializdImportedRepository
	mu              sync.Mutex
	states          map[uuid.UUID]*migrateState
	queue           *migrateQueue
}

type migrateState struct {
	progress MigrateProgress
}

func NewMigrateService(deps MigrateDeps) *MigrateService {
	return &MigrateService{
		tvtime:          deps.TVTime,
		serializd:       deps.Serializd,
		tokens:          deps.Tokens,
		showLookup:      deps.ShowLookup,
		exports:         deps.Exports,
		jobs:            deps.Jobs,
		progress:        deps.Progress,
		importedShows:   deps.ImportedShows,
		importedShowsDB: deps.ImportedShowsDB,
		states:          make(map[uuid.UUID]*migrateState),
		queue:           newMigrateQueue(),
	}
}

func (s *MigrateService) Start(ctx context.Context, req MigrateInitRequest) (uuid.UUID, error) {
	if err := s.validateCredentials(ctx, req); err != nil {
		return uuid.Nil, err
	}

	var format *tvtime.OutputFormat
	if req.DumpEnabled {
		f := req.DumpFormat
		if f == "" {
			f = tvtime.OutputFormatJSON
		}
		format = &f
	}

	jobID, err := s.jobs.Create(ctx, req.DumpEnabled, format)
	if err != nil {
		return uuid.Nil, err
	}

	state := &migrateState{
		progress: MigrateProgress{
			JobID:           jobID.String(),
			Status:          "queued",
			CurrentStep:     2,
			CurrentActivity: "Waiting in migration queue…",
			TVTimeLogin:     LoginStep{Status: StepDone},
			SerializdLogin:  LoginStep{Status: StepDone},
			GatherShows:     CountStep{Status: StepPending},
			CheckExisting:   CheckExistingStep{Status: StepPending},
			ImportShows:     ImportStep{Status: StepPending},
			Summary:         SummaryStep{Status: StepPending},
			Logs: []string{
				"TV Time credentials verified",
				"Serializd credentials verified",
			},
		},
	}
	s.setState(jobID, state)
	s.persistProgress(ctx, jobID, state, repository.MigratePending)
	s.enqueue(jobID, req)

	return jobID, nil
}

func (s *MigrateService) GetProgress(ctx context.Context, jobID uuid.UUID) (*MigrateProgress, error) {
	s.mu.Lock()
	if state, ok := s.states[jobID]; ok && state != nil {
		progress := state.progress
		s.mu.Unlock()
		if progress.Status == "queued" {
			s.enrichQueuedProgress(ctx, &progress, jobID)
		}
		return &progress, nil
	}
	s.mu.Unlock()

	var progress MigrateProgress
	ok, err := s.progress.Get(ctx, jobID, &progress)
	if err != nil {
		return nil, err
	}
	if ok {
		s.enrichQueuedProgress(ctx, &progress, jobID)
		return &progress, nil
	}

	job, err := s.jobs.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if len(job.Progress) > 0 {
		if err := json.Unmarshal(job.Progress, &progress); err != nil {
			return nil, err
		}
	}

	if progress.Status == "queued" {
		s.enrichQueuedProgress(ctx, &progress, jobID)
	}

	return &progress, nil
}

func (s *MigrateService) saveDump(ctx context.Context, tokenID uuid.UUID, format tvtime.OutputFormat, result *tvtime.ExportResult, onGather tvtime.GatherProgressFunc) (uuid.UUID, error) {
	exportID, err := s.exports.Create(ctx, tokenID, format)
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.exports.MarkRunning(ctx, exportID); err != nil {
		return uuid.Nil, err
	}
	if onGather != nil {
		onGather(tvtime.GatherProgress{Phase: tvtime.GatherPhaseDump, Done: 0, Total: len(result.Shows), Detail: "Saving TV Time dump to database…"})
	}
	if err := s.exports.SaveShows(ctx, exportID, result.Shows, s.showLookup, result.SeriesIDs, func(done, total int, name string) {
		if onGather != nil {
			detail := fmt.Sprintf("Saving to database (%d/%d)", done, total)
			if name != "" {
				detail = fmt.Sprintf("Saving to database: %s (%d/%d)", name, done, total)
			}
			onGather(tvtime.GatherProgress{Phase: tvtime.GatherPhaseDump, Done: done, Total: total, Detail: detail})
		}
	}); err != nil {
		s.markExportFailed(ctx, exportID, err)
		return uuid.Nil, err
	}
	if err := s.exports.MarkCompleted(ctx, exportID, len(result.Shows), result.WatchedEpisodes, result.DurationMs); err != nil {
		return uuid.Nil, err
	}
	return exportID, nil
}

func (s *MigrateService) persist(ctx context.Context, jobID uuid.UUID, state *migrateState, status repository.MigrateJobStatus) error {
	applog.LogBestEffort(
		s.progress.Set(ctx, jobID, state.progress),
		"migrate progress cache set",
		"job_id", jobID,
	)
	return s.jobs.UpdateProgress(ctx, jobID, status, state.progress)
}

func (s *MigrateService) persistProgress(ctx context.Context, jobID uuid.UUID, state *migrateState, status repository.MigrateJobStatus) {
	if err := s.persist(ctx, jobID, state, status); err != nil {
		applog.Error("migrate persist", "job_id", jobID, "err", err)
	}
}

func (s *MigrateService) markExportFailed(ctx context.Context, exportID uuid.UUID, cause error) {
	applog.LogBestEffort(
		s.exports.MarkFailed(ctx, exportID, cause.Error()),
		"migrate export mark failed",
		"export_id", exportID,
	)
}

func (s *MigrateService) setState(jobID uuid.UUID, state *migrateState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[jobID] = state
}

func (s *MigrateService) getState(jobID uuid.UUID) *migrateState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.states[jobID]
}

func showRef(show tvtime.ExportShow) ShowRef {
	return ShowRef{Name: showTitle(show), TVDBID: show.ID.TVDB}
}

func showTitle(show tvtime.ExportShow) string {
	if show.Title != nil {
		return *show.Title
	}
	return ""
}

func lookupInput(show tvtime.ExportShow) repository.TMDBLookupInput {
	return repository.TMDBLookupInput{
		TVDBID: show.ID.TVDB,
		IMDBID: show.ID.IMDB,
		Title:  showTitle(show),
		Year:   show.Year,
	}
}

func isCredentialError(err error) bool {
	msg := strings.ToLower(err.Error())
	isUnauthorized := strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "unauthorized")
	isLoginFailure := strings.Contains(msg, "invalid") ||
		strings.Contains(msg, "login failed")
	return isUnauthorized || isLoginFailure
}
