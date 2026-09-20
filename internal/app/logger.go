package app

import (
	"log/slog"
	"os"
)

func NewLogger(service, env, version string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return slog.New(handler).With("service", service, "env", env, "version", version)
}
