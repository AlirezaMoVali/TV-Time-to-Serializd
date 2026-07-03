package service

import (
	"context"
	"fmt"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
)

type migrateStepFail func(step, msg string)

func (s *MigrateService) run(jobID uuid.UUID, req MigrateInitRequest) {
	ctx := context.Background()
	state := s.getState(jobID)
	if state == nil {
		applog.Error("migrate state missing", "job_id", jobID)
		return
	}
	fail := s.migrateFailFunc(ctx, jobID, state)

	tokenID, tvtimeTokens, ok := s.runTVTimeLoginStep(ctx, jobID, state, req, fail)
	if !ok {
		return
	}

	serializdToken, ok := s.runSerializdLoginStep(ctx, jobID, state, req, fail)
	if !ok {
		return
	}

	result, ok := s.runGatherShowsStep(ctx, jobID, state, tvtimeTokens, fail)
	if !ok {
		return
	}

	exportID := s.runOptionalDumpStep(ctx, jobID, state, req, tokenID, result)

	pending, skippedCount, notFound, ok := s.runCheckExistingStep(
		ctx,
		jobID,
		state,
		serializdToken,
		req.SerializdEmail,
		result.Shows,
		fail,
	)
	if !ok {
		return
	}

	newlyAdded, notFound := s.runImportShowsStep(
		ctx,
		jobID,
		state,
		serializdToken,
		req.SerializdEmail,
		pending,
		notFound,
	)

	s.runCompleteStep(ctx, jobID, state, result, skippedCount, newlyAdded, notFound, exportID)
}

func (s *MigrateService) migrateFailFunc(ctx context.Context, jobID uuid.UUID, state *migrateState) migrateStepFail {
	return func(step, msg string) {
		state.progress.Status = string(repository.MigrateFailed)
		state.progress.Logs = append(state.progress.Logs, msg)
		s.persistProgress(ctx, jobID, state, repository.MigrateFailed)
		if err := s.jobs.MarkFailed(ctx, jobID, state.progress); err != nil {
			applog.Error("migrate mark failed", "job_id", jobID, "err", err)
		}
		applog.Error("migrate job failed", "job_id", jobID, "step", step, "message", msg)
	}
}

func (s *MigrateService) runTVTimeLoginStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	req MigrateInitRequest,
	fail migrateStepFail,
) (uuid.UUID, *tvtime.Tokens, bool) {
	state.progress.CurrentStep = 1
	state.progress.CurrentActivity = "Refreshing TV Time session…"
	state.progress.TVTimeLogin = LoginStep{Status: StepRunning}
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)

	tvtimeTokens, err := s.tvtime.Login(req.TVTimeEmail, req.TVTimePassword)
	if err != nil {
		state.progress.TVTimeLogin = LoginStep{Status: loginStepStatus(err), Message: err.Error()}
		fail("tvtime_login", "TV Time login failed: "+err.Error())
		return uuid.Nil, nil, false
	}

	tokenID, err := s.tokens.Upsert(ctx, req.TVTimeEmail, tvtimeTokens)
	if err != nil {
		state.progress.TVTimeLogin = LoginStep{Status: StepError, Message: err.Error()}
		fail("tvtime_login", "Failed to store TV Time session: "+err.Error())
		return uuid.Nil, nil, false
	}

	state.progress.TVTimeLogin = LoginStep{Status: StepDone}
	state.progress.Logs = append(state.progress.Logs, "TV Time session refreshed")
	state.progress.CurrentActivity = ""
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
	return tokenID, tvtimeTokens, true
}

func (s *MigrateService) runSerializdLoginStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	req MigrateInitRequest,
	fail migrateStepFail,
) (string, bool) {
	state.progress.CurrentStep = 2
	state.progress.CurrentActivity = "Refreshing Serializd session…"
	state.progress.SerializdLogin = LoginStep{Status: StepRunning}
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)

	serializdToken, err := s.serializd.Login(req.SerializdEmail, req.SerializdPassword)
	if err != nil {
		state.progress.SerializdLogin = LoginStep{Status: loginStepStatus(err), Message: err.Error()}
		fail("serializd_login", "Serializd login failed: "+err.Error())
		return "", false
	}

	state.progress.SerializdLogin = LoginStep{Status: StepDone}
	state.progress.Logs = append(state.progress.Logs, "Serializd session refreshed")
	state.progress.CurrentActivity = ""
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
	return serializdToken, true
}

func (s *MigrateService) runGatherShowsStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	tvtimeTokens *tvtime.Tokens,
	fail migrateStepFail,
) (*tvtime.ExportResult, bool) {
	state.progress.CurrentStep = 3
	state.progress.CurrentActivity = "Gathering shows and watch history from TV Time…"
	state.progress.GatherShows = CountStep{Status: StepRunning}
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)

	result, err := s.tvtime.ExportShowsWithProgress(tvtimeTokens, func(p tvtime.GatherProgress) {
		applyGatherProgress(state, p)
		if p.Done%progressPersistEvery == 0 || p.Done == p.Total || p.Phase != tvtime.GatherPhaseEpisodes {
			s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
		}
	})
	if err != nil {
		state.progress.GatherShows = CountStep{Status: StepError, Message: err.Error()}
		fail("gather_shows", "Failed to gather TV Time shows: "+err.Error())
		return nil, false
	}

	state.progress.GatherShows = CountStep{
		Status: StepDone,
		Phase:  string(tvtime.GatherPhaseEpisodes),
		Total:  len(result.Shows),
		Done:   len(result.Shows),
	}
	state.progress.CurrentActivity = ""
	state.progress.Logs = append(state.progress.Logs, fmt.Sprintf("Gathered %d shows from TV Time", len(result.Shows)))
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
	return result, true
}

func (s *MigrateService) runOptionalDumpStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	req MigrateInitRequest,
	tokenID uuid.UUID,
	result *tvtime.ExportResult,
) *uuid.UUID {
	if !req.DumpEnabled {
		return nil
	}

	format := req.DumpFormat
	if format == "" {
		format = tvtime.OutputFormatJSON
	}

	id, dumpErr := s.saveDump(ctx, tokenID, format, result, func(p tvtime.GatherProgress) {
		applyGatherProgress(state, p)
		s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
	})
	if dumpErr != nil {
		state.progress.Logs = append(state.progress.Logs, "Dump save failed: "+dumpErr.Error())
		s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
		return nil
	}

	exportID := &id
	exportStr := id.String()
	state.progress.Summary.ExportID = &exportStr
	state.progress.Logs = append(state.progress.Logs, "TV Time dump saved")
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
	return exportID
}

func (s *MigrateService) runCheckExistingStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	serializdToken string,
	serializdEmail string,
	shows []tvtime.ExportShow,
	fail migrateStepFail,
) ([]pendingImport, int, []ShowRef, bool) {
	state.progress.CurrentStep = 4
	state.progress.CurrentActivity = "Reading Serializd library and resolving show IDs…"
	state.progress.CheckExisting = CheckExistingStep{Status: StepRunning, Total: len(shows)}
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)

	pending, skippedCount, notFound, skippedRefs, checkErr := s.parallelCheckExisting(
		ctx,
		jobID,
		state,
		s.serializd,
		serializdToken,
		serializdEmail,
		shows,
	)
	if checkErr != nil {
		fail("check_existing", "Failed to "+checkErr.Error())
		return nil, 0, nil, false
	}

	applog.Info("migrate check_existing done",
		"job_id", jobID,
		"workers", lookupConcurrency(),
		"pending", len(pending),
		"skipped", skippedCount,
		"not_found", len(notFound),
	)

	state.progress.CheckExisting = CheckExistingStep{
		Status:  StepDone,
		Total:   len(shows),
		Done:    len(shows),
		Skipped: skippedRefs,
	}
	state.progress.CurrentActivity = ""
	state.progress.CurrentShow = ""
	state.progress.ActiveShows = nil
	state.progress.ImportShows = ImportStep{
		Status:    StepRunning,
		Total:     len(pending),
		Done:      0,
		Remaining: len(pending),
		NotFound:  notFound,
	}
	s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
	return pending, skippedCount, notFound, true
}

func (s *MigrateService) runImportShowsStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	serializdToken string,
	serializdEmail string,
	pending []pendingImport,
	notFound []ShowRef,
) (int, []ShowRef) {
	newlyAdded, notFound := s.parallelImportShows(ctx, jobID, state, serializdToken, serializdEmail, pending, notFound)
	applog.Info("migrate import done",
		"job_id", jobID,
		"workers", importConcurrency(),
		"added", newlyAdded,
		"not_found", len(notFound),
	)
	state.progress.ImportShows.NotFound = notFound
	state.progress.ImportShows.Status = StepDone
	state.progress.CurrentActivity = ""
	state.progress.CurrentShow = ""
	state.progress.ActiveShows = nil
	return newlyAdded, notFound
}

func (s *MigrateService) runCompleteStep(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	result *tvtime.ExportResult,
	skippedCount int,
	newlyAdded int,
	notFound []ShowRef,
	exportID *uuid.UUID,
) {
	state.progress.CurrentStep = 5
	state.progress.Summary = SummaryStep{
		Status:             StepDone,
		TotalTVTime:        len(result.Shows),
		AlreadyInSerializd: skippedCount,
		NewlyAdded:         newlyAdded,
		NotFound:           len(notFound),
		ExportID:           state.progress.Summary.ExportID,
	}
	state.progress.Status = string(repository.MigrateCompleted)
	state.progress.Logs = append(state.progress.Logs, fmt.Sprintf(
		"Migration complete: %d total, %d already in Serializd, %d newly added, %d not found",
		len(result.Shows), skippedCount, newlyAdded, len(notFound),
	))
	s.persistProgress(ctx, jobID, state, repository.MigrateCompleted)
	if err := s.jobs.MarkCompleted(ctx, jobID, state.progress, exportID); err != nil {
		applog.Error("migrate mark completed", "job_id", jobID, "err", err)
	}
}

func loginStepStatus(err error) StepStatus {
	if isCredentialError(err) {
		return StepWrongCredentials
	}
	return StepError
}
