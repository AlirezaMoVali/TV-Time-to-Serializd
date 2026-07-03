package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/serializd"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
)

const (
	defaultLookupConcurrency = 24
	defaultImportConcurrency = 8
	progressPersistEvery     = 5
)

type pendingImport struct {
	show   tvtime.ExportShow
	tmdbID int
}

type resolvedShow struct {
	show       tvtime.ExportShow
	ref        ShowRef
	tmdbID     *int
	resolveLog string
}

func lookupConcurrency() int {
	return envConcurrency("MIGRATE_LOOKUP_CONCURRENCY", defaultLookupConcurrency)
}

func importConcurrency() int {
	return envConcurrency("MIGRATE_IMPORT_CONCURRENCY", defaultImportConcurrency)
}

func envConcurrency(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	if n > 64 {
		return 64
	}
	return n
}

func (s *MigrateService) parallelResolveShows(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	shows []tvtime.ExportShow,
) []resolvedShow {
	total := len(shows)
	if total == 0 {
		return nil
	}

	workers := lookupConcurrency()
	if workers > total {
		workers = total
	}

	jobs := make(chan int, workers)
	out := make(chan resolvedShow, workers)
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	var done atomic.Int32
	tracker := newActiveShowsTracker()

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				show := shows[i]
				ref := showRef(show)
				progressMu.Lock()
				tracker.start(ref.Name)
				applyActiveShows(state, tracker)
				if name := tracker.primary(); name != "" {
					n := int(done.Load())
					state.progress.CurrentActivity = fmt.Sprintf("Checking: %s (%d/%d)", name, n, total)
				}
				progressMu.Unlock()

				item := resolvedShow{show: show, ref: ref}

				tmdbID, resolveErr := s.showLookup.ResolveTMDBID(ctx, lookupInput(show))
				if resolveErr != nil {
					item.resolveLog = fmt.Sprintf("TMDB lookup error for %s: %v", ref.Name, resolveErr)
				}
				item.tmdbID = tmdbID
				if tmdbID == nil && item.resolveLog == "" {
					item.resolveLog = fmt.Sprintf("TMDB not found: %s (tvdb=%v)", ref.Name, ref.TVDBID)
				}
				out <- item
			}
		}()
	}

	go func() {
		for i := range shows {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()

	results := make([]resolvedShow, 0, total)

	for item := range out {
		progressMu.Lock()
		tracker.done(item.ref.Name)
		results = append(results, item)
		if item.resolveLog != "" {
			state.progress.Logs = append(state.progress.Logs, item.resolveLog)
		}
		n := int(done.Add(1))
		state.progress.CheckExisting.Done = n
		state.progress.CheckExisting.Remaining = total - n
		applyActiveShows(state, tracker)
		if item.ref.Name != "" {
			state.progress.CurrentShow = item.ref.Name
		} else if name := tracker.primary(); name != "" {
			state.progress.CurrentShow = name
		}
		if name := state.progress.CurrentShow; name != "" {
			state.progress.CurrentActivity = fmt.Sprintf("Checking: %s (%d/%d)", name, n, total)
		} else {
			state.progress.CurrentActivity = fmt.Sprintf("Resolving TMDB IDs (%d/%d)", n, total)
		}
		if n%progressPersistEvery == 0 || n == total {
			s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
		}
		progressMu.Unlock()
	}

	return results
}

func classifyResolvedShows(
	resolved []resolvedShow,
	existingIDs map[int]struct{},
) (pending []pendingImport, skippedCount int, notFound []ShowRef, skipped []ShowRef) {
	pending = make([]pendingImport, 0, len(resolved))
	notFound = make([]ShowRef, 0)
	skipped = make([]ShowRef, 0)
	seenPending := make(map[int]struct{})

	for _, item := range resolved {
		if item.tmdbID == nil {
			notFound = append(notFound, item.ref)
			continue
		}
		tmdbID := *item.tmdbID
		if _, ok := existingIDs[tmdbID]; ok {
			skippedCount++
			skipped = append(skipped, item.ref)
			continue
		}
		if _, ok := seenPending[tmdbID]; ok {
			skippedCount++
			skipped = append(skipped, item.ref)
			continue
		}
		seenPending[tmdbID] = struct{}{}
		pending = append(pending, pendingImport{show: item.show, tmdbID: tmdbID})
	}

	return pending, skippedCount, notFound, skipped
}

func (s *MigrateService) parallelCheckExisting(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	serializdClient *serializd.Client,
	serializdToken string,
	serializdEmail string,
	shows []tvtime.ExportShow,
) (pending []pendingImport, skippedCount int, notFound []ShowRef, skipped []ShowRef, err error) {
	total := len(shows)
	if total == 0 {
		return nil, 0, nil, nil, nil
	}

	var (
		userInfo   *serializd.UserInformation
		userErr    error
		resolved   []resolvedShow
		parallelWg sync.WaitGroup
	)

	parallelWg.Add(2)
	go func() {
		defer parallelWg.Done()
		userInfo, userErr = serializdClient.GetUserInformation(serializdToken)
	}()
	go func() {
		defer parallelWg.Done()
		resolved = s.parallelResolveShows(ctx, jobID, state, shows)
	}()
	parallelWg.Wait()

	if userErr != nil {
		return nil, 0, nil, nil, fmt.Errorf("read Serializd account: %w", userErr)
	}

	existingIDs := serializd.ExtractTrackedShowIDs(userInfo.Context)
	if imported, impErr := s.previouslyImported(ctx, serializdEmail); impErr == nil {
		existingIDs = mergeTrackedShowIDs(existingIDs, imported)
	}
	pending, skippedCount, notFound, skipped = classifyResolvedShows(resolved, existingIDs)

	for _, item := range resolved {
		if item.tmdbID == nil {
			continue
		}
		if _, ok := existingIDs[*item.tmdbID]; ok {
			state.progress.Logs = append(state.progress.Logs,
				fmt.Sprintf("Skipped already in Serializd: %s (tvdb=%v)", item.ref.Name, item.ref.TVDBID))
		}
	}

	state.progress.CheckExisting.Skipped = skipped
	return pending, skippedCount, notFound, skipped, nil
}

type importOutcome struct {
	ref      ShowRef
	success  bool
	notFound bool
	logLine  string
}

func (s *MigrateService) parallelImportShows(
	ctx context.Context,
	jobID uuid.UUID,
	state *migrateState,
	serializdToken string,
	serializdEmail string,
	items []pendingImport,
	initialNotFound []ShowRef,
) (newlyAdded int, notFound []ShowRef) {
	total := len(items)
	notFound = append([]ShowRef(nil), initialNotFound...)
	if total == 0 {
		return 0, notFound
	}

	workers := importConcurrency()
	if workers > total {
		workers = total
	}

	jobs := make(chan pendingImport, workers)
	outcomes := make(chan importOutcome, workers)
	var wg sync.WaitGroup
	var importedMu sync.Mutex
	var progressMu sync.Mutex
	var done atomic.Int32
	tracker := newActiveShowsTracker()
	importedInRun := make(map[int]struct{}, len(items))

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				ref := showRef(item.show)

				importedMu.Lock()
				if _, ok := importedInRun[item.tmdbID]; ok {
					importedMu.Unlock()
					outcomes <- importOutcome{
						ref:     ref,
						success: true,
						logLine: fmt.Sprintf("Skipped duplicate import: %s (tmdb=%d)", ref.Name, item.tmdbID),
					}
					continue
				}
				importedInRun[item.tmdbID] = struct{}{}
				importedMu.Unlock()

				progressMu.Lock()
				tracker.start(ref.Name)
				applyActiveShows(state, tracker)
				if name := tracker.primary(); name != "" {
					n := int(done.Load())
					state.progress.CurrentActivity = fmt.Sprintf("Adding to Serializd: %s (%d/%d)", name, n, total)
				}
				progressMu.Unlock()

				if err := ImportTVTimeShow(s.serializd, serializdToken, item.tmdbID, item.show); err != nil {
					importedMu.Lock()
					delete(importedInRun, item.tmdbID)
					importedMu.Unlock()
					outcomes <- importOutcome{
						ref:      ref,
						notFound: true,
						logLine:  fmt.Sprintf("Import failed for %s (tvdb=%v): %v", ref.Name, ref.TVDBID, err),
					}
					continue
				}
				s.recordImported(ctx, serializdEmail, item.tmdbID)
				outcomes <- importOutcome{
					ref:     ref,
					success: true,
					logLine: fmt.Sprintf("Imported to Serializd (%s): %s", item.show.Status, ref.Name),
				}
			}
		}()
	}

	go func() {
		for _, item := range items {
			jobs <- item
		}
		close(jobs)
		wg.Wait()
		close(outcomes)
	}()

	for outcome := range outcomes {
		progressMu.Lock()
		tracker.done(outcome.ref.Name)
		if outcome.logLine != "" {
			state.progress.Logs = append(state.progress.Logs, outcome.logLine)
		}
		if outcome.notFound {
			notFound = append(notFound, outcome.ref)
		}
		if outcome.success {
			newlyAdded++
		}

		n := int(done.Add(1))
		state.progress.ImportShows.Done = n
		state.progress.ImportShows.Remaining = total - n
		state.progress.ImportShows.NotFound = notFound
		applyActiveShows(state, tracker)
		if outcome.success && outcome.ref.Name != "" {
			state.progress.CurrentShow = outcome.ref.Name
		} else if name := tracker.primary(); name != "" {
			state.progress.CurrentShow = name
		}
		if name := state.progress.CurrentShow; name != "" {
			state.progress.CurrentActivity = fmt.Sprintf("Adding to Serializd: %s (%d/%d)", name, n, total)
		} else {
			state.progress.CurrentActivity = fmt.Sprintf("Importing shows (%d/%d)", n, total)
		}
		if n%progressPersistEvery == 0 || n == total {
			s.persistProgress(ctx, jobID, state, repository.MigrateRunning)
		}
		progressMu.Unlock()
	}

	return newlyAdded, notFound
}
