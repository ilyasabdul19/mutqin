// api/internal/middleware/rls.go
package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// RLSTx is the minimal interface the middleware needs from a transaction:
// the ability to run "SET LOCAL app.current_tenant".
type RLSTx interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

// RLSRunner is the surface RLSContext middleware needs from a Bun handle.
// In production this is *bun.DB; tests provide a fake.
type RLSRunner interface {
	RunInTx(ctx context.Context, opts *bun.IDB, fn func(ctx context.Context, tx RLSTx) error) error
}

// RLSContext returns middleware that, when the request ctx carries a tenant,
// runs the rest of the request inside a Bun transaction whose first statement
// is `SET LOCAL app.current_tenant = '<uuid>'`. If no tenant is in ctx, the
// middleware is a no-op.
func RLSContext(runner RLSRunner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := tenant.From(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			err := runner.RunInTx(r.Context(), nil, func(ctx context.Context, tx RLSTx) error {
				if err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", id.String())); err != nil {
					return err
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil
			})
			if err != nil {
				http.Error(w, "tenant context error", http.StatusInternalServerError)
			}
		})
	}
}

// realRunner adapts *bun.DB to RLSRunner.
type realRunner struct{ db *bun.DB }

// NewBunRunner returns an RLSRunner backed by a real *bun.DB.
func NewBunRunner(db *bun.DB) RLSRunner { return &realRunner{db: db} }

func (r *realRunner) RunInTx(ctx context.Context, _ *bun.IDB, fn func(ctx context.Context, tx RLSTx) error) error {
	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, bunTx{tx: tx})
	})
}

type bunTx struct{ tx bun.Tx }

func (b bunTx) ExecContext(ctx context.Context, query string, args ...any) error {
	_, err := b.tx.ExecContext(ctx, query, args...)
	return err
}
