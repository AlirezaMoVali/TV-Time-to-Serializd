package service

import (
	"context"
	"fmt"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
)

type ExportDeps struct {
	TVTime     *tvtime.Client
	Tokens     *repository.TokenRepository
	Exports    *repository.ExportRepository
	ShowLookup repository.ShowEnsurer
}

type ExportService struct {
	tvtime     *tvtime.Client
	tokens     *repository.TokenRepository
	exports    *repository.ExportRepository
	showLookup repository.ShowEnsurer
}

func NewExportService(deps ExportDeps) *ExportService {
	return &ExportService{
		tvtime:     deps.TVTime,
		tokens:     deps.Tokens,
		exports:    deps.Exports,
		showLookup: deps.ShowLookup,
	}
}

func (s *ExportService) StartExport(ctx context.Context, tokenID uuid.UUID, format tvtime.OutputFormat) (uuid.UUID, error) {
	token, err := s.tokens.GetByID(ctx, tokenID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("token not found: %w", err)
	}

	exportID, err := s.exports.Create(ctx, tokenID, format)
	if err != nil {
		return uuid.Nil, err
	}

	go s.runExport(exportID, token.TVTimeTokens())

	return exportID, nil
}

func (s *ExportService) runExport(exportID uuid.UUID, tokens *tvtime.Tokens) {
	ctx := context.Background()

	if err := s.exports.MarkRunning(ctx, exportID); err != nil {
		applog.Error("export mark running", "export_id", exportID, "err", err)
		return
	}

	result, err := s.tvtime.ExportShows(tokens)
	if err != nil {
		s.markExportFailed(ctx, exportID, err)
		applog.Error("export tvtime fetch", "export_id", exportID, "err", err)
		return
	}

	if err := s.exports.SaveShows(ctx, exportID, result.Shows, s.showLookup, result.SeriesIDs, nil); err != nil {
		s.markExportFailed(ctx, exportID, err)
		applog.Error("export save shows", "export_id", exportID, "err", err)
		return
	}

	if err := s.exports.MarkCompleted(ctx, exportID, len(result.Shows), result.WatchedEpisodes, result.DurationMs); err != nil {
		applog.Error("export mark completed", "export_id", exportID, "err", err)
	}
}

func (s *ExportService) markExportFailed(ctx context.Context, exportID uuid.UUID, cause error) {
	applog.LogBestEffort(
		s.exports.MarkFailed(ctx, exportID, cause.Error()),
		"export mark failed",
		"export_id", exportID,
	)
}

func (s *ExportService) GetExport(ctx context.Context, exportID uuid.UUID) (*repository.UserExport, []tvtime.ExportShow, error) {
	exp, err := s.exports.GetByID(ctx, exportID)
	if err != nil {
		return nil, nil, err
	}

	if exp.Status != repository.ExportCompleted {
		return exp, nil, nil
	}

	shows, err := s.exports.LoadShows(ctx, exportID)
	if err != nil {
		return exp, nil, err
	}
	return exp, shows, nil
}

type DownloadFile struct {
	Body        []byte
	ContentType string
	Filename    string
}

func (s *ExportService) DownloadExport(ctx context.Context, exportID uuid.UUID, format tvtime.OutputFormat) (*DownloadFile, error) {
	exp, shows, err := s.GetExport(ctx, exportID)
	if err != nil {
		return nil, err
	}
	if exp.Status != repository.ExportCompleted {
		return nil, ErrExportNotCompleted
	}

	date := exp.StartedAt.UTC().Format("2006-01-02")
	if exp.CompletedAt != nil {
		date = exp.CompletedAt.UTC().Format("2006-01-02")
	}

	switch format {
	case tvtime.OutputFormatJSON:
		body, err := tvtime.ShowsToJSON(shows)
		if err != nil {
			return nil, err
		}
		return &DownloadFile{
			Body:        body,
			ContentType: "application/json",
			Filename:    fmt.Sprintf("tvtime-series-%s.json", date),
		}, nil
	case tvtime.OutputFormatCSV:
		body, err := tvtime.ShowsToCSV(shows)
		if err != nil {
			return nil, err
		}
		return &DownloadFile{
			Body:        body,
			ContentType: "text/csv",
			Filename:    fmt.Sprintf("tvtime-series-%s.csv", date),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported download format")
	}
}
