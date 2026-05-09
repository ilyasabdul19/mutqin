// api/internal/service/halaqah_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubHalaqahRepo struct {
	stored map[uuid.UUID]*model.Halaqah
	listed []model.Halaqah
	// captured args for inspection
	lastListLimit  int
	lastListOffset int
}

func newStubHalaqahRepo() *stubHalaqahRepo {
	return &stubHalaqahRepo{stored: map[uuid.UUID]*model.Halaqah{}}
}

func (s *stubHalaqahRepo) Create(_ context.Context, h *model.Halaqah) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	cp := *h
	s.stored[h.ID] = &cp
	return nil
}

func (s *stubHalaqahRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Halaqah, error) {
	if h, ok := s.stored[id]; ok {
		cp := *h
		return &cp, nil
	}
	return nil, repo.ErrNotFound
}

func (s *stubHalaqahRepo) ListByOrg(_ context.Context, limit, offset int) ([]model.Halaqah, error) {
	s.lastListLimit = limit
	s.lastListOffset = offset
	return s.listed, nil
}

func (s *stubHalaqahRepo) Update(_ context.Context, h *model.Halaqah) error {
	if _, ok := s.stored[h.ID]; !ok {
		return repo.ErrNotFound
	}
	cp := *h
	s.stored[h.ID] = &cp
	return nil
}

func TestHalaqahService_Create_RecordsAudit(t *testing.T) {
	halaqat := newStubHalaqahRepo()
	audit := &stubAuditRepo{}
	svc := service.NewHalaqahService(halaqat, audit)

	actor := uuid.New()
	orgID := uuid.New()
	h, err := svc.Create(context.Background(), actor, orgID, service.CreateHalaqahInput{
		Name: "Halaqah Al-Asr", MaxCapacity: 25,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h == nil {
		t.Fatal("nil halaqah")
	}
	if h.OrganizationID != orgID {
		t.Fatalf("org id = %s want %s", h.OrganizationID, orgID)
	}
	if h.Name != "Halaqah Al-Asr" {
		t.Fatalf("name = %s", h.Name)
	}
	if h.MaxCapacity != 25 {
		t.Fatalf("max capacity = %d", h.MaxCapacity)
	}
	if h.Status != "active" {
		t.Fatalf("status = %s", h.Status)
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs = %d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "create_halaqah" {
		t.Fatalf("action = %s", audit.logs[0].Action)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("actor mismatch")
	}
}

func TestHalaqahService_Create_DefaultsMaxCapacity(t *testing.T) {
	halaqat := newStubHalaqahRepo()
	svc := service.NewHalaqahService(halaqat, &stubAuditRepo{})

	h, err := svc.Create(context.Background(), uuid.New(), uuid.New(), service.CreateHalaqahInput{
		Name: "X",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h.MaxCapacity != 30 {
		t.Fatalf("max capacity = %d want 30 (default)", h.MaxCapacity)
	}
}

func TestHalaqahService_Update_RecordsAudit(t *testing.T) {
	halaqat := newStubHalaqahRepo()
	audit := &stubAuditRepo{}
	svc := service.NewHalaqahService(halaqat, audit)

	actor := uuid.New()
	orgID := uuid.New()
	h, err := svc.Create(context.Background(), actor, orgID, service.CreateHalaqahInput{Name: "Original", MaxCapacity: 20})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	h.Name = "Updated"
	h.MaxCapacity = 40
	if err := svc.Update(context.Background(), actor, h); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(audit.logs) != 2 {
		t.Fatalf("audit logs = %d want 2", len(audit.logs))
	}
	if audit.logs[1].Action != "update_halaqah" {
		t.Fatalf("action = %s want update_halaqah", audit.logs[1].Action)
	}

	got, err := svc.GetByID(context.Background(), h.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Updated" {
		t.Fatalf("name = %s want Updated", got.Name)
	}
	if got.MaxCapacity != 40 {
		t.Fatalf("max capacity = %d want 40", got.MaxCapacity)
	}
}

func TestHalaqahService_Deactivate_RecordsAudit(t *testing.T) {
	halaqat := newStubHalaqahRepo()
	audit := &stubAuditRepo{}
	svc := service.NewHalaqahService(halaqat, audit)

	actor := uuid.New()
	orgID := uuid.New()
	h, err := svc.Create(context.Background(), actor, orgID, service.CreateHalaqahInput{Name: "X", MaxCapacity: 10})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Deactivate(context.Background(), actor, h.ID); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	got, err := svc.GetByID(context.Background(), h.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "inactive" {
		t.Fatalf("status = %s want inactive", got.Status)
	}
	if len(audit.logs) != 2 {
		t.Fatalf("audit logs = %d want 2", len(audit.logs))
	}
	if audit.logs[1].Action != "deactivate_halaqah" {
		t.Fatalf("action = %s want deactivate_halaqah", audit.logs[1].Action)
	}
}

func TestHalaqahService_List_ClampsLimit(t *testing.T) {
	halaqat := newStubHalaqahRepo()
	svc := service.NewHalaqahService(halaqat, &stubAuditRepo{})

	if _, err := svc.List(context.Background(), 0, 0); err != nil {
		t.Fatalf("List(0): %v", err)
	}
	if halaqat.lastListLimit != 50 {
		t.Fatalf("limit=0 → %d want 50", halaqat.lastListLimit)
	}

	if _, err := svc.List(context.Background(), 500, 0); err != nil {
		t.Fatalf("List(500): %v", err)
	}
	if halaqat.lastListLimit != 50 {
		t.Fatalf("limit=500 → %d want 50 (clamped)", halaqat.lastListLimit)
	}

	if _, err := svc.List(context.Background(), 25, 10); err != nil {
		t.Fatalf("List(25,10): %v", err)
	}
	if halaqat.lastListLimit != 25 || halaqat.lastListOffset != 10 {
		t.Fatalf("limit=%d offset=%d want 25/10", halaqat.lastListLimit, halaqat.lastListOffset)
	}
}
