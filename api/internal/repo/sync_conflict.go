// api/internal/repo/sync_conflict.go
package repo

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type SyncConflictRepo struct{ db bun.IDB }

func NewSyncConflictRepo(d bun.IDB) *SyncConflictRepo { return &SyncConflictRepo{db: d} }

// idb returns the request-scoped tx from ctx if RLSContext middleware has set
// one (so SET LOCAL app.current_tenant applies); otherwise returns the handle
// the repo was constructed with. Required for RLS-subject tables.
func (r *SyncConflictRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

// Create inserts a sync-conflict audit row.
func (r *SyncConflictRepo) Create(ctx context.Context, c *model.SyncConflict) error {
	_, err := r.idb(ctx).NewInsert().Model(c).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert sync_conflict: %w", err)
	}
	return nil
}
