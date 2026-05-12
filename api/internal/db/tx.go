// api/internal/db/tx.go
package db

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// RunInTx executes fn inside a database transaction. The transaction commits on
// nil return; rolls back on error. Repo methods accept bun.IDB so they work
// against either *bun.DB or bun.Tx. Must be called with a top-level *bun.DB;
// pass bun.Tx to repo methods directly when composing nested logical units.
func RunInTx(ctx context.Context, bdb *bun.DB, fn func(ctx context.Context, tx bun.Tx) error) error {
	return bdb.RunInTx(ctx, &sql.TxOptions{}, fn)
}
