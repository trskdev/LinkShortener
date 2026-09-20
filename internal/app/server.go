package app

import (
	"errors"
	"log/slog"
	"net/http"
	"pht/pet/link_shortener/pkg/config"
)

func StartHttpServer(logger *slog.Logger, cfg config.ServerConfig, mux http.Handler, errCh chan<- error) *http.Server {
	addr := cfg.GetAddr()

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	go func() {
		logger.Info("server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			errCh <- err
		}
	}()

	return srv
}
