package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"pht/pet/link_shortener/internal/app"
	"pht/pet/link_shortener/internal/database"
	"pht/pet/link_shortener/internal/domain/db"
	"pht/pet/link_shortener/internal/handler"
	"pht/pet/link_shortener/internal/repository"
	"pht/pet/link_shortener/internal/service"
	"pht/pet/link_shortener/pkg/cache"
	"pht/pet/link_shortener/pkg/config"
	"sync"
	"syscall"
	"time"
)

const serviceName = "link-shortener"
const serviceVersion = "0.1.0"

func main() {
	cfg := config.LoadConfig()

	logger := app.NewLogger(serviceName, cfg.Env, serviceVersion)

	ctx := context.Background()

	errCh := make(chan error, 1)

	notifyCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	pool, err := database.InitDB(ctx, logger, cfg.DB)
	if err != nil {
		logger.Error("pgx pool init failed", "error", err)
		return
	}
	defer pool.Close()

	redis, err := cache.NewRedisClient(cfg.Redis, logger)
	if err != nil {
		logger.Error("redis init failed", "error", err)
		return
	}
	defer redis.Close()

	mux := http.NewServeMux()

	queries := db.New(pool)
	linkRepo := repository.NewPGXURLRepository(queries)
	linkService := service.NewLinkService(linkRepo, redis.Client, logger)
	linkHandler := handler.NewLinkHandler(linkService, logger)

	linkService.StartClickSyncWorker(notifyCtx, 60*time.Second, &wg)

	mux.Handle(
		"POST /shorten",
		http.HandlerFunc(linkHandler.Create),
	)

	mux.Handle(
		"GET /{code}",
		http.HandlerFunc(linkHandler.GetURL),
	)

	mux.Handle(
		"GET /stats/{code}",
		http.HandlerFunc(linkHandler.GetClicks),
	)

	srv := app.StartHttpServer(logger, cfg.Srv, mux, errCh)

	select {
	case <-notifyCtx.Done():
		logger.Info("received a shutdown signal. Shutting down")
	case err := <-errCh:
		logger.Error("critical server error. Shutting down", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown the server", "error", err)
	}

	close(errCh)
	for err := range errCh {
		logger.Error("error captured during the shutdown", "error", err)
	}

	wg.Wait()

	logger.Info("server stopped")
}
