// api/internal/repo/repo_test.go
package repo_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
)

var testDB *bun.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("mutqin_test"),
		tcpostgres.WithUsername("mutqin"),
		tcpostgres.WithPassword("mutqin"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start postgres testcontainer: %v", err)
	}
	defer func() {
		if err := pgC.Terminate(ctx); err != nil {
			log.Printf("terminate testcontainer: %v", err)
		}
	}()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	bdb, err := db.NewDB(ctx, dsn, false)
	if err != nil {
		log.Fatalf("connect bun: %v", err)
	}
	defer bdb.Close()

	if err := migrate.Up(ctx, bdb); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	testDB = bdb
	os.Exit(m.Run())
}
