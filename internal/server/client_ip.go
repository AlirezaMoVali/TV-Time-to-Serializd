package server

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func useClientIP(r interface {
	Use(middlewares ...func(http.Handler) http.Handler)
}) {
	r.Use(middleware.ClientIPFromRemoteAddr)
	if n := envInt("TRUSTED_PROXY_COUNT", 1); n >= 1 {
		r.Use(middleware.ClientIPFromXFFTrustedProxies(n))
	}
}

func clientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
