// api/internal/repo/platform_stats.go
package repo

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// PlatformStatsRepo runs cross-tenant aggregate queries for the super-admin
// dashboard. It is constructed with the admin (superuser) handle so RLS
// policies are bypassed and rows from every organization are counted.
type PlatformStatsRepo struct {
	db bun.IDB
}

func NewPlatformStatsRepo(d bun.IDB) *PlatformStatsRepo {
	return &PlatformStatsRepo{db: d}
}

// PlatformStats is the rollup returned by GET /platform/stats.
type PlatformStats struct {
	Centers             int `json:"centers"`
	ActiveCenters       int `json:"active_centers"`
	SuspendedCenters    int `json:"suspended_centers"`
	TotalStudents       int `json:"total_students"`
	TotalRecitations30d int `json:"total_recitations_30d"`
}

// Compute returns a snapshot of platform-wide counters. Uses raw QueryRowContext
// on the underlying handle directly (no tenant ctx, no RLS filter).
func (r *PlatformStatsRepo) Compute(ctx context.Context) (*PlatformStats, error) {
	s := &PlatformStats{}
	row := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM organizations) AS centers,
		  (SELECT COUNT(*) FROM organizations WHERE status='active') AS active,
		  (SELECT COUNT(*) FROM organizations WHERE status='suspended') AS suspended,
		  (SELECT COUNT(*) FROM students WHERE status='active') AS students,
		  (SELECT COUNT(*) FROM recitations WHERE recorded_at >= now() - interval '30 days') AS rec30d
	`)
	if err := row.Scan(&s.Centers, &s.ActiveCenters, &s.SuspendedCenters, &s.TotalStudents, &s.TotalRecitations30d); err != nil {
		return nil, fmt.Errorf("platform stats: %w", err)
	}
	return s, nil
}
