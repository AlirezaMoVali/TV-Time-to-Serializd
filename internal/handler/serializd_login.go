package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/cache"
	"github.com/alireza/tvtime2serializd/internal/repository"
	"github.com/alireza/tvtime2serializd/internal/serializd"
)

type SerializdLoginDeps struct {
	Client   *serializd.Client
	Tokens   *repository.SerializdTokenRepository
	Sessions *cache.SerializdSessionCache
}

type SerializdLogin struct {
	client   *serializd.Client
	tokens   *repository.SerializdTokenRepository
	sessions *cache.SerializdSessionCache
}

func NewSerializdLogin(deps SerializdLoginDeps) *SerializdLogin {
	return &SerializdLogin{
		client:   deps.Client,
		tokens:   deps.Tokens,
		sessions: deps.Sessions,
	}
}

type serializdLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type serializdLoginResponse struct {
	ID string `json:"id"`
}

func (h *SerializdLogin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req serializdLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Email == "" {
		req.Email = os.Getenv("SERIALIZD_EMAIL")
	}
	if req.Password == "" {
		req.Password = os.Getenv("SERIALIZD_PASSWORD")
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	jwtToken, err := h.client.Login(req.Email, req.Password)
	if err != nil {
		respondError(w, http.StatusBadGateway, "login failed", err, "operation", "serializd_login")
		return
	}

	tokenID, err := h.tokens.Upsert(r.Context(), req.Email, jwtToken)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save token", err, "operation", "serializd_login_persist")
		return
	}

	if err := h.sessions.Set(r.Context(), tokenID, jwtToken); err != nil {
		applog.LogBestEffort(err, "cache serializd session", "token_id", tokenID)
	}

	writeJSON(w, http.StatusOK, serializdLoginResponse{ID: tokenID.String()})
}
