//go:build integration

package repository

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatal(err)
	}

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	mig, err := migrate.New("file://../../migrations", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal(err)
	}

	pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()

	pool.Close()
	pg.Terminate(ctx)
	os.Exit(code)
}

func truncateAll(t *testing.T) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"TRUNCATE links RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatal(err)
	}
}
