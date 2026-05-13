// api/internal/service/announcement_test.go
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

type stubAnnouncementRepo struct {
	stored         map[uuid.UUID]*model.Announcement
	lastListLimit  int
	lastListOffset int
}

func newStubAnnouncementRepo() *stubAnnouncementRepo {
	return &stubAnnouncementRepo{stored: map[uuid.UUID]*model.Announcement{}}
}

func (s *stubAnnouncementRepo) Create(_ context.Context, a *model.Announcement) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	cp := *a
	s.stored[a.ID] = &cp
	return nil
}

func (s *stubAnnouncementRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Announcement, error) {
	if a, ok := s.stored[id]; ok {
		cp := *a
		return &cp, nil
	}
	return nil, repo.ErrNotFound
}

func (s *stubAnnouncementRepo) ListByOrg(_ context.Context, limit, offset int) ([]model.Announcement, error) {
	s.lastListLimit = limit
	s.lastListOffset = offset
	var out []model.Announcement
	for _, a := range s.stored {
		out = append(out, *a)
	}
	return out, nil
}

func (s *stubAnnouncementRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := s.stored[id]; !ok {
		return repo.ErrNotFound
	}
	delete(s.stored, id)
	return nil
}

func TestAnnouncementService_Create_RecordsAudit(t *testing.T) {
	repoStub := newStubAnnouncementRepo()
	audit := &stubAuditRepo{}
	svc := service.NewAnnouncementService(repoStub, audit)

	orgID := uuid.New()
	actor := uuid.New()
	a, err := svc.Create(context.Background(), actor, orgID, service.CreateAnnouncementInput{
		Title: "Eid Mubarak",
		Body:  "Classes resume Monday.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.OrganizationID != orgID {
		t.Fatalf("org id mismatch")
	}
	if a.Title != "Eid Mubarak" || a.Body != "Classes resume Monday." {
		t.Fatalf("title/body mismatch: %+v", a)
	}
	if len(audit.logs) != 1 || audit.logs[0].Action != "create_announcement" {
		t.Fatalf("audit: %+v", audit.logs)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("audit actor mismatch")
	}
}

func TestAnnouncementService_Create_RejectsBlankTitle(t *testing.T) {
	repoStub := newStubAnnouncementRepo()
	audit := &stubAuditRepo{}
	svc := service.NewAnnouncementService(repoStub, audit)

	_, err := svc.Create(context.Background(), uuid.New(), uuid.New(), service.CreateAnnouncementInput{
		Title: "  ",
		Body:  "x",
	})
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("err=%v want ErrInvalidInput", err)
	}
	if len(repoStub.stored) != 0 {
		t.Fatalf("stored=%d want 0", len(repoStub.stored))
	}
	if len(audit.logs) != 0 {
		t.Fatalf("audit on validation failure")
	}
}

func TestAnnouncementService_Create_RejectsBlankBody(t *testing.T) {
	svc := service.NewAnnouncementService(newStubAnnouncementRepo(), &stubAuditRepo{})
	_, err := svc.Create(context.Background(), uuid.New(), uuid.New(), service.CreateAnnouncementInput{
		Title: "x",
		Body:  "   ",
	})
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("err=%v want ErrInvalidInput", err)
	}
}

func TestAnnouncementService_Delete_RecordsAudit(t *testing.T) {
	repoStub := newStubAnnouncementRepo()
	audit := &stubAuditRepo{}
	svc := service.NewAnnouncementService(repoStub, audit)

	actor := uuid.New()
	a, err := svc.Create(context.Background(), actor, uuid.New(), service.CreateAnnouncementInput{
		Title: "t", Body: "b",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(context.Background(), actor, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := repoStub.stored[a.ID]; ok {
		t.Fatal("announcement still stored after delete")
	}
	if len(audit.logs) != 2 {
		t.Fatalf("audit logs=%d want 2 (create+delete)", len(audit.logs))
	}
	if audit.logs[1].Action != "delete_announcement" {
		t.Fatalf("second audit action=%s", audit.logs[1].Action)
	}
}

func TestAnnouncementService_ListByOrg_ClampsLimit(t *testing.T) {
	repoStub := newStubAnnouncementRepo()
	svc := service.NewAnnouncementService(repoStub, &stubAuditRepo{})

	if _, err := svc.ListByOrg(context.Background(), 0, 0); err != nil {
		t.Fatalf("ListByOrg(0): %v", err)
	}
	if repoStub.lastListLimit != 50 {
		t.Fatalf("limit=0 → %d want 50", repoStub.lastListLimit)
	}
	if _, err := svc.ListByOrg(context.Background(), 500, 7); err != nil {
		t.Fatalf("ListByOrg(500): %v", err)
	}
	if repoStub.lastListLimit != 50 {
		t.Fatalf("limit=500 → %d want 50 (clamped)", repoStub.lastListLimit)
	}
	if repoStub.lastListOffset != 7 {
		t.Fatalf("offset = %d want 7", repoStub.lastListOffset)
	}
}
