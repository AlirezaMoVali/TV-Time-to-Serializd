package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthDeps struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

type Health struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewHealth(deps HealthDeps) *Health {
	return &Health{db: deps.DB, redis: deps.Redis}
}

type healthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	services := map[string]string{
		"postgres": "ok",
		"redis":    "ok",
	}
	status := "ok"
	code := http.StatusOK

	if err := h.db.Ping(ctx); err != nil {
		services["postgres"] = "down"
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		services["redis"] = "down"
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, healthResponse{
		Status:   status,
		Services: services,
	})
}
