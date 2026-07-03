package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
)

type LoginDeps struct {
	TVTime   *tvtime.Client
	Tokens   *repository.TokenRepository
	Sessions *cache.SessionCache
}

type Login struct {
	tvtime   *tvtime.Client
	tokens   *repository.TokenRepository
	sessions *cache.SessionCache
}

func NewLogin(deps LoginDeps) *Login {
	return &Login{
		tvtime:   deps.TVTime,
		tokens:   deps.Tokens,
		sessions: deps.Sessions,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	ID string `json:"id"`
}

func (h *Login) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Email == "" {
		req.Email = os.Getenv("TVTIME_EMAIL")
	}
	if req.Password == "" {
		req.Password = os.Getenv("TVTIME_PASSWORD")
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	tvtimeTokens, err := h.tvtime.Login(req.Email, req.Password)
	if err != nil {
		respondError(w, http.StatusBadGateway, "login failed", err, "operation", "tvtime_login")
		return
	}

	tokenID, err := h.tokens.Upsert(r.Context(), req.Email, tvtimeTokens)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save tokens", err, "operation", "tvtime_login_persist")
		return
	}

	if err := h.sessions.Set(r.Context(), tokenID, tvtimeTokens); err != nil {
		applog.LogBestEffort(err, "cache tvtime session", "token_id", tokenID)
	}

	writeJSON(w, http.StatusOK, loginResponse{ID: tokenID.String()})
}

// GetTokenID parses a token UUID from a path or header value.
func GetTokenID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}
