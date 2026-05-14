// api/internal/service/sync.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

// ErrInvalidSyncInput is returned when the sync payload fails validation.
var ErrInvalidSyncInput = errors.New("service: invalid sync input")

type recitationSync interface {
	BatchCreate(ctx context.Context, rs []model.Recitation) error
	ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Recitation, error)
}

type attendanceSync interface {
	UpsertBatch(ctx context.Context, rs []model.Attendance) error
	ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Attendance, error)
}

type syncConflictSink interface {
	Create(ctx context.Context, c *model.SyncConflict) error
}

// SyncService coordinates offline-batch Push + watermark Pull. Recitations
// use insert-only BatchCreate (client_id-based dedupe is future work).
// Attendance uses UpsertBatch (server-idempotent via UNIQUE(student_id, date)).
type SyncService struct {
	recs     recitationSync
	att      attendanceSync
	conflict syncConflictSink
	audit    auditRepo
}

func NewSyncService(r recitationSync, a attendanceSync, c syncConflictSink, audit auditRepo) *SyncService {
	return &SyncService{recs: r, att: a, conflict: c, audit: audit}
}

// PushInput is the validated push payload (a batch of offline writes).
type PushInput struct {
	Recitations []model.Recitation
	Attendance  []model.Attendance
}

// PushResult reports how many rows of each kind were accepted.
type PushResult struct {
	RecitationsAccepted int       `json:"recitations_accepted"`
	AttendanceAccepted  int       `json:"attendance_accepted"`
	ServerNow           time.Time `json:"server_now"`
}

// Push stamps server-trusted org + teacher onto every row (clients can lie
// about org) and dispatches to the underlying repos. Both writes run on the
// same request-scoped tx if RLSContext middleware set one in ctx.
func (s *SyncService) Push(ctx context.Context, actorID, orgID uuid.UUID, in PushInput) (*PushResult, error) {
	now := time.Now()
	for i := range in.Recitations {
		in.Recitations[i].OrganizationID = orgID
		in.Recitations[i].TeacherID = actorID
		t := now
		in.Recitations[i].SyncedAt = &t
	}
	for i := range in.Attendance {
		in.Attendance[i].OrganizationID = orgID
		t := now
		in.Attendance[i].SyncedAt = &t
	}

	if len(in.Recitations) > 0 {
		if err := s.recs.BatchCreate(ctx, in.Recitations); err != nil {
			return nil, fmt.Errorf("push recitations: %w", err)
		}
	}
	if len(in.Attendance) > 0 {
		if err := s.att.UpsertBatch(ctx, in.Attendance); err != nil {
			return nil, fmt.Errorf("push attendance: %w", err)
		}
	}

	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID: &actorID,
		Action:  "sync_push",
	})

	return &PushResult{
		RecitationsAccepted: len(in.Recitations),
		AttendanceAccepted:  len(in.Attendance),
		ServerNow:           time.Now().UTC(),
	}, nil
}

// PullResult is the response for /sync/pull.
type PullResult struct {
	Recitations []model.Recitation `json:"recitations"`
	Attendance  []model.Attendance `json:"attendance"`
	ServerNow   time.Time          `json:"server_now"`
}

// Pull emits all rows whose recorded_at > since for the tenant on ctx.
func (s *SyncService) Pull(ctx context.Context, since time.Time, limit int) (*PullResult, error) {
	recs, err := s.recs.ListByOrgSince(ctx, since, limit)
	if err != nil {
		return nil, fmt.Errorf("pull recitations: %w", err)
	}
	att, err := s.att.ListByOrgSince(ctx, since, limit)
	if err != nil {
		return nil, fmt.Errorf("pull attendance: %w", err)
	}
	return &PullResult{Recitations: recs, Attendance: att, ServerNow: time.Now().UTC()}, nil
}
