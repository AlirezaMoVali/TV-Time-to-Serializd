package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/service"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Migrate struct {
	migrate *service.MigrateService
}

func NewMigrate(migrate *service.MigrateService) *Migrate {
	return &Migrate{migrate: migrate}
}

type migrateInitRequest struct {
	TVTimeEmail       string `json:"tvtime_email"`
	TVTimePassword    string `json:"tvtime_password"`
	SerializdEmail    string `json:"serializd_email"`
	SerializdPassword string `json:"serializd_password"`
	Dump              struct {
		Enabled bool   `json:"enabled"`
		Format  string `json:"format,omitempty"`
	} `json:"dump"`
}

type migrateInitResponse struct {
	ID string `json:"id"`
}

func (h *Migrate) Init(w http.ResponseWriter, r *http.Request) {
	var req migrateInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.TVTimeEmail == "" || req.TVTimePassword == "" || req.SerializdEmail == "" || req.SerializdPassword == "" {
		writeError(w, http.StatusBadRequest, "tvtime and serializd credentials are required")
		return
	}

	format, err := tvtime.ParseOutputFormat(req.Dump.Format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	jobID, err := h.migrate.Start(r.Context(), service.MigrateInitRequest{
		TVTimeEmail:       req.TVTimeEmail,
		TVTimePassword:    req.TVTimePassword,
		SerializdEmail:    req.SerializdEmail,
		SerializdPassword: req.SerializdPassword,
		DumpEnabled:       req.Dump.Enabled,
		DumpFormat:        format,
	})
	if err != nil {
		if isCredentialValidationError(err) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to start migration", err, "operation", "migrate_init")
		return
	}

	writeJSON(w, http.StatusOK, migrateInitResponse{ID: jobID.String()})
}

func (h *Migrate) Get(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	progress, err := h.migrate.GetProgress(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "migration job not found")
		return
	}
	writeJSON(w, http.StatusOK, progress)
}

func (h *Migrate) Stream(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var last string
	for {
		progress, err := h.migrate.GetProgress(r.Context(), jobID)
		if err != nil {
			if _, writeErr := io.WriteString(w, "event: error\ndata: {\"error\":\"job not found\"}\n\n"); writeErr != nil {
				applog.LogBestEffort(writeErr, "write migrate stream error", "job_id", jobID)
			}
			flusher.Flush()
			return
		}

		payload, err := json.Marshal(progress)
		if err != nil {
			applog.Error("marshal migrate progress", "job_id", jobID, "err", err)
			return
		}
		current := string(payload)
		if current != last {
			if _, writeErr := fmt.Fprintf(w, "event: progress\ndata: %s\n\n", current); writeErr != nil {
				applog.LogBestEffort(writeErr, "write migrate stream progress", "job_id", jobID)
				return
			}
			flusher.Flush()
			last = current
		}

		if progress.Status == "completed" || progress.Status == "failed" {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, writeErr := io.WriteString(w, ": heartbeat\n\n"); writeErr != nil {
				applog.LogBestEffort(writeErr, "write migrate stream heartbeat", "job_id", jobID)
				return
			}
			flusher.Flush()
		}
	}
}

func isCredentialValidationError(err error) bool {
	var credErr *service.CredentialValidationError
	return errors.As(err, &credErr)
}
