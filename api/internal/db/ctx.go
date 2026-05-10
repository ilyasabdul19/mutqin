// api/internal/db/ctx.go
package db

import (
	"context"

	"github.com/uptrace/bun"
)

// txKey is the ctx key used to attach the per-request transaction installed
// by middleware.RLSContext. When a repo method runs within an RLS-scoped
// request, the bun.IDB stored under this key is the transaction whose first
// statement was `SET LOCAL app.current_tenant = '<uuid>'`. Repo methods that
// touch RLS-subject tables MUST grab their handle via TxFrom so the policy
// fires on the correct connection.
type txKey struct{}

// WithTx returns a child ctx carrying the request-scoped transaction.
func WithTx(ctx context.Context, tx bun.IDB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFrom returns the request-scoped transaction from ctx, or fallback if none
// is attached. Repo methods write `db.TxFrom(ctx, r.db)` instead of `r.db` so
// they automatically pick up the RLS transaction when one exists.
func TxFrom(ctx context.Context, fallback bun.IDB) bun.IDB {
	if tx, ok := ctx.Value(txKey{}).(bun.IDB); ok {
		return tx
	}
	return fallback
}
