// api/internal/db/tx.go
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

// RunInTx executes fn inside a database transaction. The transaction commits on
// nil return; rolls back on error. Repo methods accept bun.IDB so they work
// against either *bun.DB or bun.Tx.
func RunInTx(ctx context.Context, db *bun.DB, fn func(ctx context.Context, tx bun.Tx) error) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := fn(ctx, tx); err != nil {
			return fmt.Errorf("tx fn: %w", err)
		}
		return nil
	})
}
