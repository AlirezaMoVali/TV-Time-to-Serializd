package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alireza/tvtime2serializd/internal/applog"
	"github.com/alireza/tvtime2serializd/internal/config"
	"github.com/alireza/tvtime2serializd/internal/platform/postgres"
	redisClient "github.com/alireza/tvtime2serializd/internal/platform/redis"
	"github.com/alireza/tvtime2serializd/internal/serializd"
	"github.com/alireza/tvtime2serializd/internal/server"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	applog.Configure()
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := redisClient.NewClient(cfg.RedisURL)
	if err != nil {
		return err
	}
	defer rdb.Close()

	resolved, negative, err := server.WarmTMDBCache(ctx, pool, rdb)
	if err != nil {
		applog.Warn("tmdb cache warm failed", "err", err)
	} else if resolved > 0 || negative > 0 {
		applog.Info("tmdb cache warmed from postgres", "resolved", resolved, "negative", negative)
	}

	srv, err := server.New(server.Deps{
		DB:                 pool,
		Redis:              rdb,
		TVTime:             tvtime.NewClient(),
		Serializd:          serializd.NewClient(),
		TokenEncryptionKey: cfg.TokenEncryptionKey,
		TMDBAPIKey:         cfg.TMDBAPIKey,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	})
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// WriteTimeout disabled so SSE streams and long exports are not cut off.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		applog.Info("listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		applog.Info("shutting down", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return httpServer.Shutdown(shutdownCtx)
}
