package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/alireza/tvtime2serializd/internal/applog"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		applog.Error("encode json response", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		applog.LogBestEffort(err, "write json response")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// respondError logs technical details once at the HTTP boundary and returns a safe user message.
func respondError(w http.ResponseWriter, status int, userMessage string, err error, attrs ...any) {
	if err != nil {
		args := append(attrs, slog.Any("err", err))
		applog.Error("request failed", args...)
	}
	writeError(w, status, userMessage)
}
