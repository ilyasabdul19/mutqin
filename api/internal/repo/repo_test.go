// api/internal/repo/repo_test.go
package repo_test

import (
	"context"
	"fmt"
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

var (
	testDB    *bun.DB // mutqin_app connection — RLS-subject, TenantHook installed
	testAdmin *bun.DB // mutqin superuser — no RLS, no hook (used by truncateAll and admin lookups)
)

// truncateAll empties every tenant-bearing table. Tests should call it via
// t.Cleanup so each test sees a known-empty database. Uses the admin handle so
// it bypasses RLS.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testAdmin.ExecContext(context.Background(),
		`TRUNCATE TABLE otp_codes, invites, users, organizations RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

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

	host, err := pgC.Host(ctx)
	if err != nil {
		log.Fatalf("container host: %v", err)
	}
	port, err := pgC.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("container port: %v", err)
	}

	adminDSN := fmt.Sprintf("postgres://mutqin:mutqin@%s:%s/mutqin_test?sslmode=disable", host, port.Port())
	appDSN := fmt.Sprintf("postgres://mutqin_app:mutqin_app@%s:%s/mutqin_test?sslmode=disable", host, port.Port())

	admin, err := db.NewDB(ctx, adminDSN, false)
	if err != nil {
		log.Fatalf("connect admin: %v", err)
	}
	defer admin.Close()
	testAdmin = admin

	if err := migrate.Up(ctx, admin); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	app, err := db.NewDB(ctx, appDSN, false, db.TenantHook{})
	if err != nil {
		log.Fatalf("connect app: %v", err)
	}
	defer app.Close()
	testDB = app

	os.Exit(m.Run())
}
