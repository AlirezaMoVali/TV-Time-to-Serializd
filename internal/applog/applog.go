package applog

import (
	"log/slog"
	"os"
)

// Configure installs the process-wide structured logger.
func Configure() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
}

// Info logs at info level with structured attributes.
func Info(msg string, args ...any) {
	slog.Default().Info(msg, args...)
}

// Warn logs at warn level with structured attributes.
func Warn(msg string, args ...any) {
	slog.Default().Warn(msg, args...)
}

// Error logs at error level with structured attributes.
func Error(msg string, args ...any) {
	slog.Default().Error(msg, args...)
}

// LogBestEffort logs a non-fatal error without affecting the caller's control flow.
func LogBestEffort(err error, msg string, args ...any) {
	if err == nil {
		return
	}
	attrs := append(args, slog.Any("err", err))
	slog.Default().Warn(msg, attrs...)
}
