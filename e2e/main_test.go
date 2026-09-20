//go:build integration

package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var appURL string

func TestMain(m *testing.M) {
	code := runTests(m)

	os.Exit(code)
}

func runTests(m *testing.M) int {
	ctx := context.Background()

	net, err := network.New(ctx)
	if err != nil {
		panic(err)
	}
	network := net.Name

	defer net.Remove(ctx)

	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Networks: []string{network},
				NetworkAliases: map[string][]string{
					network: {"postgres-db"},
				},
			},
		}),
	)
	if err != nil {
		panic(err)
	}

	defer pg.Terminate(ctx)

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}
	mig, err := migrate.New("file://../migrations", dsn)
	if err != nil {
		panic(err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic(err)
	}

	redisContainer, err := redis.Run(ctx, "redis:7-alpine",
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Networks: []string{network},
				NetworkAliases: map[string][]string{
					network: {"redis-cache"},
				},
			},
		}),
	)
	if err != nil {
		panic(err)
	}

	defer redisContainer.Terminate(ctx)

	appReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../",
			Dockerfile: "Dockerfile",
		},
		Env: map[string]string{
			"DB_HOST":    "postgres-db",
			"DB_PORT":    "5432",
			"DB_USER":    "test",
			"DB_PWD":     "test",
			"DB_NAME":    "test",
			"REDIS_HOST": "redis-cache",
			"REDIS_PORT": "6379",
			"ENV":        "dev",
		},
		ExposedPorts: []string{"8080/tcp"},
		Networks:     []string{network},
		WaitingFor:   wait.ForListeningPort("8080/tcp").WithStartupTimeout(30 * time.Second),
	}

	appContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: appReq,
		Started:          true,
	})
	if err != nil {
		panic(err)
	}
	defer appContainer.Terminate(ctx)

	host, err := appContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := appContainer.MappedPort(ctx, "8080")
	if err != nil {
		panic(err)
	}

	appURL = fmt.Sprintf("http://%s:%s", host, port.Port())

	return m.Run()
}
