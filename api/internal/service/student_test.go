// api/internal/service/student_test.go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubStudentRepo struct {
	stored map[uuid.UUID]*model.Student
	// captured args for inspection
	lastListLimit  int
	lastListOffset int
}

func newStubStudentRepo() *stubStudentRepo {
	return &stubStudentRepo{stored: map[uuid.UUID]*model.Student{}}
}

func (s *stubStudentRepo) Create(_ context.Context, st *model.Student) error {
	if st.ID == uuid.Nil {
		st.ID = uuid.New()
	}
	cp := *st
	s.stored[st.ID] = &cp
	return nil
}

func (s *stubStudentRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Student, error) {
	if st, ok := s.stored[id]; ok {
		cp := *st
		return &cp, nil
	}
	return nil, repo.ErrNotFound
}

func (s *stubStudentRepo) ListByHalaqah(_ context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error) {
	s.lastListLimit = limit
	s.lastListOffset = offset
	var out []model.Student
	for _, st := range s.stored {
		if st.HalaqahID != nil && *st.HalaqahID == halaqahID {
			out = append(out, *st)
		}
	}
	return out, nil
}

func (s *stubStudentRepo) ListByOrg(_ context.Context, limit, offset int) ([]model.Student, error) {
	s.lastListLimit = limit
	s.lastListOffset = offset
	var out []model.Student
	for _, st := range s.stored {
		out = append(out, *st)
	}
	return out, nil
}

func (s *stubStudentRepo) Update(_ context.Context, st *model.Student) error {
	if _, ok := s.stored[st.ID]; !ok {
		return repo.ErrNotFound
	}
	cp := *st
	s.stored[st.ID] = &cp
	return nil
}

func (s *stubStudentRepo) Transfer(_ context.Context, studentID, newHalaqahID uuid.UUID) error {
	st, ok := s.stored[studentID]
	if !ok {
		return repo.ErrNotFound
	}
	st.HalaqahID = &newHalaqahID
	return nil
}

// stubHalaqahReader is a read-only halaqah lookup mock for cross-tenant validation.
type stubHalaqahReader struct {
	stored map[uuid.UUID]*model.Halaqah
}

func newStubHalaqahReader() *stubHalaqahReader {
	return &stubHalaqahReader{stored: map[uuid.UUID]*model.Halaqah{}}
}

func (s *stubHalaqahReader) GetByID(_ context.Context, id uuid.UUID) (*model.Halaqah, error) {
	if h, ok := s.stored[id]; ok {
		cp := *h
		return &cp, nil
	}
	return nil, repo.ErrNotFound
}

func TestStudentService_Enroll_RecordsAuditAndAttachesHalaqah(t *testing.T) {
	students := newStubStudentRepo()
	halaqat := newStubHalaqahReader()
	audit := &stubAuditRepo{}
	svc := service.NewStudentService(students, halaqat, audit)

	orgID := uuid.New()
	halaqahID := uuid.New()
	halaqat.stored[halaqahID] = &model.Halaqah{ID: halaqahID, OrganizationID: orgID, Name: "H"}

	actor := uuid.New()
	st, err := svc.Enroll(context.Background(), actor, orgID, halaqahID, service.EnrollStudentInput{
		Name: "Ali",
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if st.OrganizationID != orgID {
		t.Fatalf("org id = %s want %s", st.OrganizationID, orgID)
	}
	if st.HalaqahID == nil || *st.HalaqahID != halaqahID {
		t.Fatalf("halaqah id mismatch")
	}
	if st.Status != "active" {
		t.Fatalf("status = %s", st.Status)
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs = %d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "enroll_student" {
		t.Fatalf("action = %s", audit.logs[0].Action)
	}
}

func TestStudentService_Enroll_RejectsCrossTenant(t *testing.T) {
	students := newStubStudentRepo()
	halaqat := newStubHalaqahReader()
	audit := &stubAuditRepo{}
	svc := service.NewStudentService(students, halaqat, audit)

	orgA := uuid.New()
	orgB := uuid.New()
	halaqahID := uuid.New()
	// halaqah belongs to orgB, but caller claims orgA
	halaqat.stored[halaqahID] = &model.Halaqah{ID: halaqahID, OrganizationID: orgB, Name: "H"}

	_, err := svc.Enroll(context.Background(), uuid.New(), orgA, halaqahID, service.EnrollStudentInput{
		Name: "Ali",
	})
	if !errors.Is(err, service.ErrCrossTenant) {
		t.Fatalf("err = %v want ErrCrossTenant", err)
	}
	if len(students.stored) != 0 {
		t.Fatalf("students stored = %d want 0", len(students.stored))
	}
	if len(audit.logs) != 0 {
		t.Fatalf("audit on cross-tenant rejection")
	}
}

func TestStudentService_Transfer_RecordsAudit(t *testing.T) {
	students := newStubStudentRepo()
	halaqat := newStubHalaqahReader()
	audit := &stubAuditRepo{}
	svc := service.NewStudentService(students, halaqat, audit)

	orgID := uuid.New()
	halaqahA := uuid.New()
	halaqahB := uuid.New()
	halaqat.stored[halaqahA] = &model.Halaqah{ID: halaqahA, OrganizationID: orgID}
	halaqat.stored[halaqahB] = &model.Halaqah{ID: halaqahB, OrganizationID: orgID}

	actor := uuid.New()
	st, err := svc.Enroll(context.Background(), actor, orgID, halaqahA, service.EnrollStudentInput{Name: "X"})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	if err := svc.Transfer(context.Background(), actor, st.ID, halaqahB, orgID); err != nil {
		t.Fatalf("Transfer: %v", err)
	}

	got, err := students.GetByID(context.Background(), st.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.HalaqahID == nil || *got.HalaqahID != halaqahB {
		t.Fatalf("halaqah after transfer = %v want %s", got.HalaqahID, halaqahB)
	}

	if len(audit.logs) != 2 {
		t.Fatalf("audit logs = %d want 2 (enroll + transfer)", len(audit.logs))
	}
	if audit.logs[1].Action != "transfer_student" {
		t.Fatalf("action = %s want transfer_student", audit.logs[1].Action)
	}
}

func TestStudentService_Transfer_RejectsCrossTenant(t *testing.T) {
	students := newStubStudentRepo()
	halaqat := newStubHalaqahReader()
	svc := service.NewStudentService(students, halaqat, &stubAuditRepo{})

	orgA := uuid.New()
	orgB := uuid.New()
	halaqahA := uuid.New()
	halaqahForeign := uuid.New()
	halaqat.stored[halaqahA] = &model.Halaqah{ID: halaqahA, OrganizationID: orgA}
	halaqat.stored[halaqahForeign] = &model.Halaqah{ID: halaqahForeign, OrganizationID: orgB}

	st, err := svc.Enroll(context.Background(), uuid.New(), orgA, halaqahA, service.EnrollStudentInput{Name: "X"})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	err = svc.Transfer(context.Background(), uuid.New(), st.ID, halaqahForeign, orgA)
	if !errors.Is(err, service.ErrCrossTenant) {
		t.Fatalf("err = %v want ErrCrossTenant", err)
	}
	got, _ := students.GetByID(context.Background(), st.ID)
	if got.HalaqahID == nil || *got.HalaqahID != halaqahA {
		t.Fatalf("halaqah after rejected transfer = %v want %s", got.HalaqahID, halaqahA)
	}
}

func TestStudentService_Deactivate_RecordsAudit(t *testing.T) {
	students := newStubStudentRepo()
	halaqat := newStubHalaqahReader()
	audit := &stubAuditRepo{}
	svc := service.NewStudentService(students, halaqat, audit)

	orgID := uuid.New()
	halaqahID := uuid.New()
	halaqat.stored[halaqahID] = &model.Halaqah{ID: halaqahID, OrganizationID: orgID}

	actor := uuid.New()
	st, err := svc.Enroll(context.Background(), actor, orgID, halaqahID, service.EnrollStudentInput{Name: "Y"})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	if err := svc.Deactivate(context.Background(), actor, st.ID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	got, _ := students.GetByID(context.Background(), st.ID)
	if got.Status != "inactive" {
		t.Fatalf("status = %s want inactive", got.Status)
	}
	if len(audit.logs) != 2 {
		t.Fatalf("audit logs = %d want 2", len(audit.logs))
	}
	if audit.logs[1].Action != "deactivate_student" {
		t.Fatalf("action = %s want deactivate_student", audit.logs[1].Action)
	}
}

func TestStudentService_ListByHalaqah_ClampsLimit(t *testing.T) {
	students := newStubStudentRepo()
	halaqat := newStubHalaqahReader()
	svc := service.NewStudentService(students, halaqat, &stubAuditRepo{})

	hID := uuid.New()
	if _, err := svc.ListByHalaqah(context.Background(), hID, 0, 0); err != nil {
		t.Fatalf("ListByHalaqah(0): %v", err)
	}
	if students.lastListLimit != 50 {
		t.Fatalf("limit=0 → %d want 50", students.lastListLimit)
	}
	if _, err := svc.ListByHalaqah(context.Background(), hID, 500, 5); err != nil {
		t.Fatalf("ListByHalaqah(500): %v", err)
	}
	if students.lastListLimit != 50 {
		t.Fatalf("limit=500 → %d want 50 (clamped)", students.lastListLimit)
	}
	if students.lastListOffset != 5 {
		t.Fatalf("offset = %d want 5", students.lastListOffset)
	}
}
