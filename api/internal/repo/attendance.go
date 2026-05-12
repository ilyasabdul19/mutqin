// api/internal/repo/attendance.go
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type AttendanceRepo struct {
	db bun.IDB
}

func NewAttendanceRepo(d bun.IDB) *AttendanceRepo {
	return &AttendanceRepo{db: d}
}

// idb returns the request-scoped tx from ctx if RLSContext middleware has set
// one (so SET LOCAL app.current_tenant applies); otherwise returns the handle
// the repo was constructed with. Required for RLS-subject tables.
func (r *AttendanceRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

// UpsertBatch inserts attendance rows, or on conflict (student_id, date)
// updates the status/recorded_at/synced_at fields. Idempotent.
func (r *AttendanceRepo) UpsertBatch(ctx context.Context, rows []model.Attendance) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := r.idb(ctx).NewInsert().
		Model(&rows).
		On("CONFLICT (student_id, date) DO UPDATE").
		Set("status = EXCLUDED.status").
		Set("recorded_at = EXCLUDED.recorded_at").
		Set("synced_at = EXCLUDED.synced_at").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert attendance: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error) {
	var rows []model.Attendance
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("halaqah_id = ?", halaqahID).
		Where("date = ?", date.Format("2006-01-02")).
		OrderExpr("recorded_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attendance by halaqah/date: %w", err)
	}
	return rows, nil
}

func (r *AttendanceRepo) ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	var rows []model.Attendance
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("student_id = ?", studentID).
		Where("date >= ?", from.Format("2006-01-02")).
		Where("date <= ?", to.Format("2006-01-02")).
		OrderExpr("date DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attendance by student: %w", err)
	}
	return rows, nil
}

func (r *AttendanceRepo) ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	var rows []model.Attendance
	q := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("date >= ?", from.Format("2006-01-02")).
		Where("date <= ?", to.Format("2006-01-02"))
	if halaqahID != nil {
		q = q.Where("halaqah_id = ?", *halaqahID)
	}
	err := q.OrderExpr("date DESC").Limit(limit).Offset(offset).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attendance by org: %w", err)
	}
	return rows, nil
}
