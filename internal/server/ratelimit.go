package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultCredentialRPM  = 6
	defaultCredentialBurst = 3
)

type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	limit    rate.Limit
	burst    int
}

func newCredentialRateLimiter() *ipRateLimiter {
	rpm := envInt("RATE_LIMIT_CREDENTIAL_RPM", defaultCredentialRPM)
	if rpm < 1 {
		rpm = defaultCredentialRPM
	}
	burst := envInt("RATE_LIMIT_CREDENTIAL_BURST", defaultCredentialBurst)
	if burst < 1 {
		burst = defaultCredentialBurst
	}

	return &ipRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		limit:    rate.Every(time.Minute / time.Duration(rpm)),
		burst:    burst,
	}
}

func (l *ipRateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.getLimiter(clientIP(r)).Allow() {
			writeRateLimitError(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter, ok := l.limiters[ip]
	if !ok {
		limiter = rate.NewLimiter(l.limit, l.burst)
		l.limiters[ip] = limiter
	}
	return limiter
}

func writeRateLimitError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many requests"})
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
