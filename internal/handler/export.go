package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/service"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Export struct {
	exports *service.ExportService
}

func NewExport(exports *service.ExportService) *Export {
	return &Export{exports: exports}
}

type exportRequest struct {
	TokenID string `json:"token_id"`
	Format  string `json:"format,omitempty"`
}

type exportCreateResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Format string `json:"format"`
}

type exportGetResponse struct {
	ID              string              `json:"id"`
	Status          string              `json:"status"`
	Format          string              `json:"format"`
	ShowCount       int                 `json:"show_count,omitempty"`
	WatchedEpisodes int                 `json:"watched_episodes,omitempty"`
	DurationMs      *int64              `json:"duration_ms,omitempty"`
	Error           *string             `json:"error,omitempty"`
	Shows           []tvtime.ExportShow `json:"shows,omitempty"`
}

func (h *Export) Create(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	tokenID, err := uuid.Parse(req.TokenID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "token_id must be a valid UUID")
		return
	}

	format, err := tvtime.ParseOutputFormat(req.Format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	exportID, err := h.exports.StartExport(r.Context(), tokenID, format)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to start export", err, "operation", "start_export")
		return
	}

	writeJSON(w, http.StatusOK, exportCreateResponse{
		ID:     exportID.String(),
		Status: string(repository.ExportRunning),
		Format: string(format),
	})
}

func (h *Export) Get(w http.ResponseWriter, r *http.Request) {
	exportID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid export id")
		return
	}

	exp, shows, err := h.exports.GetExport(r.Context(), exportID)
	if err != nil {
		respondError(w, http.StatusNotFound, "export not found", err, "operation", "get_export", "export_id", exportID)
		return
	}

	resp := exportGetResponse{
		ID:              exp.ID.String(),
		Status:          string(exp.Status),
		Format:          string(exp.OutputFormat),
		ShowCount:       exp.ShowCount,
		WatchedEpisodes: exp.WatchedEpisodes,
		DurationMs:      exp.DurationMs,
		Error:           exp.ErrorMessage,
	}
	if exp.Status == repository.ExportCompleted {
		resp.Shows = shows
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Export) Download(w http.ResponseWriter, r *http.Request) {
	exportID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid export id")
		return
	}

	formatParam := r.URL.Query().Get("format")
	if formatParam == "" {
		writeError(w, http.StatusBadRequest, "format query parameter is required (json or csv)")
		return
	}
	if formatParam == "both" {
		writeError(w, http.StatusBadRequest, "download one file at a time: use format=json or format=csv")
		return
	}

	format, err := tvtime.ParseOutputFormat(formatParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if format == tvtime.OutputFormatBoth {
		writeError(w, http.StatusBadRequest, "download one file at a time: use format=json or format=csv")
		return
	}

	file, err := h.exports.DownloadExport(r.Context(), exportID, format)
	if err != nil {
		if errors.Is(err, service.ErrExportNotCompleted) {
			respondError(w, http.StatusConflict, "export is not completed yet", err, "operation", "download_export", "export_id", exportID)
			return
		}
		respondError(w, http.StatusNotFound, "export not found", err, "operation", "download_export", "export_id", exportID)
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(file.Body); err != nil {
		applog.LogBestEffort(err, "write export download", "export_id", exportID)
	}
}
