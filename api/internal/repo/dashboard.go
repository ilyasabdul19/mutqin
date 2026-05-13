// api/internal/repo/dashboard.go
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
)

// DashboardRepo runs center-scoped aggregate queries. All queries are raw SQL
// (no bun model hooks fire) and rely on Postgres RLS to filter rows by tenant.
// Callers must therefore route through r.idb(ctx) so the per-request tx — with
// SET LOCAL app.current_tenant — is used.
type DashboardRepo struct {
	db bun.IDB
}

func NewDashboardRepo(d bun.IDB) *DashboardRepo {
	return &DashboardRepo{db: d}
}

func (r *DashboardRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

// CenterStats is the rollup returned by GET /dashboard/stats.
type CenterStats struct {
	Students          int     `json:"students"`
	Halaqat           int     `json:"halaqat"`
	Teachers          int     `json:"teachers"`
	Recitations30d    int     `json:"recitations_30d"`
	AttendanceRate30d float64 `json:"attendance_rate_30d"` // 0..1
}

// CenterStats computes counts for the current tenant. All tables are RLS-subject
// (organization_id filter is enforced by Postgres) so no explicit WHERE clauses
// are needed beyond the status/date filters that scope each subquery.
func (r *DashboardRepo) CenterStats(ctx context.Context) (*CenterStats, error) {
	stats := &CenterStats{}
	row := r.idb(ctx).QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM students  WHERE status = 'active') AS students,
		  (SELECT COUNT(*) FROM halaqat   WHERE status = 'active') AS halaqat,
		  (SELECT COUNT(*) FROM users     WHERE role = 'teacher' AND status = 'active') AS teachers,
		  (SELECT COUNT(*) FROM recitations WHERE recorded_at >= now() - interval '30 days') AS recitations_30d,
		  COALESCE((
		    SELECT AVG(CASE WHEN status='present' THEN 1.0 ELSE 0.0 END)
		    FROM attendance
		    WHERE date >= (now() - interval '30 days')::date
		  ), 0) AS attendance_rate_30d
	`)
	if err := row.Scan(&stats.Students, &stats.Halaqat, &stats.Teachers, &stats.Recitations30d, &stats.AttendanceRate30d); err != nil {
		return nil, fmt.Errorf("center stats: %w", err)
	}
	return stats, nil
}

// AttendanceTrendRow is one date's roll-up returned by GET /dashboard/attendance-trends.
type AttendanceTrendRow struct {
	Date         time.Time `json:"date"`
	PresentCount int       `json:"present_count"`
	AbsentCount  int       `json:"absent_count"`
	Total        int       `json:"total"`
}

// AttendanceTrends returns one row per date in [from, to]. Optional halaqahID
// narrows the rollup to a single halaqah.
func (r *DashboardRepo) AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]AttendanceTrendRow, error) {
	rows := make([]AttendanceTrendRow, 0)

	query := `
		SELECT date,
		       SUM(CASE WHEN status='present' THEN 1 ELSE 0 END) AS present,
		       SUM(CASE WHEN status='absent'  THEN 1 ELSE 0 END) AS absent,
		       COUNT(*) AS total
		FROM attendance
		WHERE date >= ? AND date <= ?
	`
	args := []any{from.Format("2006-01-02"), to.Format("2006-01-02")}
	if halaqahID != nil {
		query += " AND halaqah_id = ?"
		args = append(args, *halaqahID)
	}
	query += " GROUP BY date ORDER BY date"

	r2, err := r.idb(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("attendance trends: %w", err)
	}
	defer r2.Close()
	for r2.Next() {
		var (
			row     AttendanceTrendRow
			dateStr string
		)
		if err := r2.Scan(&dateStr, &row.PresentCount, &row.AbsentCount, &row.Total); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		d, perr := time.Parse("2006-01-02", dateStr)
		if perr != nil {
			return nil, fmt.Errorf("parse date %q: %w", dateStr, perr)
		}
		row.Date = d
		rows = append(rows, row)
	}
	if err := r2.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return rows, nil
}

// RecitationActivityRow is one halaqah's count in the activity rollup.
type RecitationActivityRow struct {
	HalaqahID   uuid.UUID `json:"halaqah_id"`
	HalaqahName string    `json:"halaqah_name"`
	Count       int       `json:"count"`
}

// RecitationActivity returns counts per halaqah for recitations recorded in
// [from, to]. Halaqat with zero recitations are omitted (INNER JOIN semantics).
func (r *DashboardRepo) RecitationActivity(ctx context.Context, from, to time.Time) ([]RecitationActivityRow, error) {
	rows := make([]RecitationActivityRow, 0)
	r2, err := r.idb(ctx).QueryContext(ctx, `
		SELECT h.id, h.name, COUNT(rec.id) AS cnt
		FROM halaqat h
		LEFT JOIN recitations rec
		  ON rec.halaqah_id = h.id
		 AND rec.recorded_at >= ?
		 AND rec.recorded_at <= ?
		GROUP BY h.id, h.name
		HAVING COUNT(rec.id) > 0
		ORDER BY cnt DESC, h.name ASC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("recitation activity: %w", err)
	}
	defer r2.Close()
	for r2.Next() {
		var row RecitationActivityRow
		if err := r2.Scan(&row.HalaqahID, &row.HalaqahName, &row.Count); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rows = append(rows, row)
	}
	if err := r2.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return rows, nil
}
