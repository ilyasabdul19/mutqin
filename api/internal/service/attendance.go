// api/internal/service/attendance.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

// ErrInvalidAttendance is returned when MarkBatch receives an empty or
// malformed payload. Handlers map this to 400 via errors.Is.
var ErrInvalidAttendance = errors.New("service: invalid attendance input")

type attendanceRepoIface interface {
	UpsertBatch(ctx context.Context, rows []model.Attendance) error
	ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error)
	ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error)
	ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error)
}

type AttendanceService struct {
	att   attendanceRepoIface
	audit auditRepo
}

func NewAttendanceService(att attendanceRepoIface, audit auditRepo) *AttendanceService {
	return &AttendanceService{att: att, audit: audit}
}

// MarkInput represents one student's attendance for a date.
type MarkInput struct {
	StudentID uuid.UUID
	Status    string // "present" | "absent"
	ClientID  *string
}

// MarkBatch upserts attendance for a halaqah on a given date. Idempotent —
// re-running with different statuses overrides the previous values.
func (s *AttendanceService) MarkBatch(ctx context.Context, actorID, orgID, halaqahID uuid.UUID, date time.Time, marks []MarkInput) ([]model.Attendance, error) {
	if len(marks) == 0 {
		return nil, fmt.Errorf("%w: empty marks", ErrInvalidAttendance)
	}
	rows := make([]model.Attendance, 0, len(marks))
	for i, m := range marks {
		if m.StudentID == uuid.Nil {
			return nil, fmt.Errorf("%w: mark %d: student_id required", ErrInvalidAttendance, i)
		}
		if m.Status != "present" && m.Status != "absent" {
			return nil, fmt.Errorf("%w: mark %d: status must be present or absent", ErrInvalidAttendance, i)
		}
		rows = append(rows, model.Attendance{
			OrganizationID: orgID,
			HalaqahID:      halaqahID,
			StudentID:      m.StudentID,
			Date:           date,
			Status:         m.Status,
			ClientID:       m.ClientID,
		})
	}
	if err := s.att.UpsertBatch(ctx, rows); err != nil {
		return nil, fmt.Errorf("upsert: %w", err)
	}

	tt := "attendance"
	tid := halaqahID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "mark_attendance_batch",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return rows, nil
}

func (s *AttendanceService) ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error) {
	return s.att.ListByHalaqahDate(ctx, halaqahID, date)
}

func (s *AttendanceService) ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.att.ListByStudent(ctx, studentID, from, to, limit, offset)
}

func (s *AttendanceService) ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.att.ListByOrg(ctx, halaqahID, from, to, limit, offset)
}
