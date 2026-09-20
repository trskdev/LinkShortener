package database

import (
	"context"
	"log/slog"
	"pht/pet/link_shortener/pkg/config"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(baseCtx context.Context, logger *slog.Logger, DBConfig config.DBConfig) (*pgxpool.Pool, error) {
	url := DBConfig.GetDSN()

	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		logger.Error("pgx config parsing failed", "error", err)
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		logger.Error("pgx pool opening failed", "error", err)
		return nil, err
	}

	return pool, nil
}
