// api/internal/db/conn.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

// NewDB opens a Bun DB backed by pgdriver, verifies connectivity, and returns
// the handle. The caller is responsible for closing it.
//
// debug=true installs bundebug to log every query — only enable in development.
func NewDB(ctx context.Context, databaseURL string, debug bool) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(30 * time.Minute)

	bdb := bun.NewDB(sqldb, pgdialect.New())
	if debug {
		bdb.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := bdb.PingContext(pingCtx); err != nil {
		_ = bdb.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return bdb, nil
}
