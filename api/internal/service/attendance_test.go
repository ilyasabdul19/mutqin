// api/internal/service/attendance_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubAttendanceRepo struct {
	upserted []model.Attendance
	upsertErr error

	listHalaqahDateReturn []model.Attendance
	listByStudentReturn   []model.Attendance
	listByOrgReturn       []model.Attendance

	// captured args
	lastStudentLimit  int
	lastStudentOffset int
	lastOrgLimit      int
	lastOrgHalaqah    *uuid.UUID
}

func (s *stubAttendanceRepo) UpsertBatch(_ context.Context, rows []model.Attendance) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, rows...)
	return nil
}

func (s *stubAttendanceRepo) ListByHalaqahDate(_ context.Context, _ uuid.UUID, _ time.Time) ([]model.Attendance, error) {
	return s.listHalaqahDateReturn, nil
}

func (s *stubAttendanceRepo) ListByStudent(_ context.Context, _ uuid.UUID, _, _ time.Time, limit, offset int) ([]model.Attendance, error) {
	s.lastStudentLimit = limit
	s.lastStudentOffset = offset
	return s.listByStudentReturn, nil
}

func (s *stubAttendanceRepo) ListByOrg(_ context.Context, halaqahID *uuid.UUID, _, _ time.Time, limit, _ int) ([]model.Attendance, error) {
	s.lastOrgLimit = limit
	s.lastOrgHalaqah = halaqahID
	return s.listByOrgReturn, nil
}

func TestAttendanceService_MarkBatch_HappyPathAndAudit(t *testing.T) {
	att := &stubAttendanceRepo{}
	audit := &stubAuditRepo{}
	svc := service.NewAttendanceService(att, audit)

	actor := uuid.New()
	orgID := uuid.New()
	halaqahID := uuid.New()
	stu1 := uuid.New()
	stu2 := uuid.New()
	date := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)

	rows, err := svc.MarkBatch(context.Background(), actor, orgID, halaqahID, date, []service.MarkInput{
		{StudentID: stu1, Status: "present"},
		{StudentID: stu2, Status: "absent"},
	})
	if err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d want 2", len(rows))
	}
	if len(att.upserted) != 2 {
		t.Fatalf("upserted=%d want 2", len(att.upserted))
	}
	if att.upserted[0].OrganizationID != orgID {
		t.Fatalf("org id mismatch")
	}
	if att.upserted[0].HalaqahID != halaqahID {
		t.Fatalf("halaqah id mismatch")
	}
	if att.upserted[0].Status != "present" || att.upserted[1].Status != "absent" {
		t.Fatalf("status order: %s/%s", att.upserted[0].Status, att.upserted[1].Status)
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "mark_attendance_batch" {
		t.Fatalf("action=%s", audit.logs[0].Action)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("actor mismatch")
	}
}

func TestAttendanceService_MarkBatch_Empty_Invalid(t *testing.T) {
	svc := service.NewAttendanceService(&stubAttendanceRepo{}, &stubAuditRepo{})
	_, err := svc.MarkBatch(context.Background(), uuid.New(), uuid.New(), uuid.New(), time.Now(), nil)
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, service.ErrInvalidAttendance) {
		t.Fatalf("err=%v want ErrInvalidAttendance", err)
	}
}

func TestAttendanceService_MarkBatch_InvalidStatus(t *testing.T) {
	svc := service.NewAttendanceService(&stubAttendanceRepo{}, &stubAuditRepo{})
	_, err := svc.MarkBatch(context.Background(), uuid.New(), uuid.New(), uuid.New(), time.Now(), []service.MarkInput{
		{StudentID: uuid.New(), Status: "maybe"},
	})
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, service.ErrInvalidAttendance) {
		t.Fatalf("err=%v want ErrInvalidAttendance", err)
	}
}

func TestAttendanceService_MarkBatch_StudentIDRequired(t *testing.T) {
	svc := service.NewAttendanceService(&stubAttendanceRepo{}, &stubAuditRepo{})
	_, err := svc.MarkBatch(context.Background(), uuid.New(), uuid.New(), uuid.New(), time.Now(), []service.MarkInput{
		{StudentID: uuid.Nil, Status: "present"},
	})
	if !errors.Is(err, service.ErrInvalidAttendance) {
		t.Fatalf("err=%v want ErrInvalidAttendance", err)
	}
}

func TestAttendanceService_ListByStudent_ClampsLimit(t *testing.T) {
	att := &stubAttendanceRepo{}
	svc := service.NewAttendanceService(att, &stubAuditRepo{})

	if _, err := svc.ListByStudent(context.Background(), uuid.New(), time.Now(), time.Now(), 0, 0); err != nil {
		t.Fatalf("ListByStudent(0): %v", err)
	}
	if att.lastStudentLimit != 50 {
		t.Fatalf("limit=0 → %d want 50", att.lastStudentLimit)
	}

	if _, err := svc.ListByStudent(context.Background(), uuid.New(), time.Now(), time.Now(), 500, 0); err != nil {
		t.Fatalf("ListByStudent(500): %v", err)
	}
	if att.lastStudentLimit != 50 {
		t.Fatalf("limit=500 → %d want 50 (clamped)", att.lastStudentLimit)
	}

	if _, err := svc.ListByStudent(context.Background(), uuid.New(), time.Now(), time.Now(), 25, 10); err != nil {
		t.Fatalf("ListByStudent(25,10): %v", err)
	}
	if att.lastStudentLimit != 25 || att.lastStudentOffset != 10 {
		t.Fatalf("limit=%d offset=%d want 25/10", att.lastStudentLimit, att.lastStudentOffset)
	}
}

func TestAttendanceService_ListByOrg_HalaqahFilterPassthrough(t *testing.T) {
	att := &stubAttendanceRepo{}
	svc := service.NewAttendanceService(att, &stubAuditRepo{})

	// Without halaqah filter.
	if _, err := svc.ListByOrg(context.Background(), nil, time.Now(), time.Now(), 25, 0); err != nil {
		t.Fatalf("ListByOrg no halaqah: %v", err)
	}
	if att.lastOrgHalaqah != nil {
		t.Fatalf("halaqah=%v want nil", att.lastOrgHalaqah)
	}
	if att.lastOrgLimit != 25 {
		t.Fatalf("limit=%d want 25", att.lastOrgLimit)
	}

	// With halaqah filter.
	h := uuid.New()
	if _, err := svc.ListByOrg(context.Background(), &h, time.Now(), time.Now(), 0, 0); err != nil {
		t.Fatalf("ListByOrg with halaqah: %v", err)
	}
	if att.lastOrgHalaqah == nil || *att.lastOrgHalaqah != h {
		t.Fatalf("halaqah=%v want %s", att.lastOrgHalaqah, h)
	}
	if att.lastOrgLimit != 50 {
		t.Fatalf("limit=0 → %d want 50", att.lastOrgLimit)
	}
}
