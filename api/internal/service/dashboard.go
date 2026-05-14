// api/internal/service/dashboard.go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

// dashboardRepoIface is the surface DashboardService needs from
// *repo.DashboardRepo; tests inject a stub.
type dashboardRepoIface interface {
	CenterStats(ctx context.Context) (*repo.CenterStats, error)
	AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error)
	RecitationActivity(ctx context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error)
}

// DashboardService exposes the center-admin dashboard endpoints. All methods
// delegate straight to the repo; the service layer exists so handlers don't
// depend on *repo and to leave a hook for future business logic (caching,
// pre-computation) without touching call sites.
type DashboardService struct {
	repo dashboardRepoIface
}

func NewDashboardService(r dashboardRepoIface) *DashboardService {
	return &DashboardService{repo: r}
}

func (s *DashboardService) CenterStats(ctx context.Context) (*repo.CenterStats, error) {
	return s.repo.CenterStats(ctx)
}

func (s *DashboardService) AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error) {
	return s.repo.AttendanceTrends(ctx, halaqahID, from, to)
}

func (s *DashboardService) RecitationActivity(ctx context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error) {
	return s.repo.RecitationActivity(ctx, from, to)
}

// platformRepoIface is the surface PlatformService needs from
// *repo.PlatformStatsRepo.
type platformRepoIface interface {
	Compute(ctx context.Context) (*repo.PlatformStats, error)
}

// auditLister is the surface PlatformService needs from *repo.AuditLogRepo.
type auditLister interface {
	ListByActor(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error)
}

// PlatformService backs the super-admin dashboard. It does NOT know about
// tenant context — the underlying repo runs with the admin handle and reads
// across every organization.
type PlatformService struct {
	stats platformRepoIface
	audit auditLister
}

func NewPlatformService(s platformRepoIface, a auditLister) *PlatformService {
	return &PlatformService{stats: s, audit: a}
}

func (s *PlatformService) Stats(ctx context.Context) (*repo.PlatformStats, error) {
	return s.stats.Compute(ctx)
}

// AuditLog returns audit rows for the given actor. Limit is clamped to
// [1, 200] with a default of 50 so callers cannot accidentally pull the
// whole table or pass zero and get nothing back.
func (s *PlatformService) AuditLog(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.audit.ListByActor(ctx, actorID, limit, offset)
}
