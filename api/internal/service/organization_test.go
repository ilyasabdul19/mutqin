// api/internal/service/organization_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubOrgRepo struct {
	created *model.Organization
	stored  map[string]*model.Organization
	byID    map[uuid.UUID]*model.Organization
	listed  []model.Organization
}

func newStubOrgRepo() *stubOrgRepo {
	return &stubOrgRepo{
		stored: map[string]*model.Organization{},
		byID:   map[uuid.UUID]*model.Organization{},
	}
}

func (s *stubOrgRepo) Create(_ context.Context, o *model.Organization) error {
	o.ID = uuid.New()
	s.created = o
	s.stored[o.Slug] = o
	s.byID[o.ID] = o
	return nil
}
func (s *stubOrgRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Organization, error) {
	if o, ok := s.byID[id]; ok {
		return o, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubOrgRepo) GetBySlug(_ context.Context, slug string) (*model.Organization, error) {
	if o, ok := s.stored[slug]; ok {
		return o, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubOrgRepo) List(_ context.Context, limit, offset int) ([]model.Organization, error) {
	return s.listed, nil
}
func (s *stubOrgRepo) Update(_ context.Context, o *model.Organization) error {
	cur, ok := s.byID[o.ID]
	if !ok {
		return repo.ErrNotFound
	}
	cur.Description = o.Description
	cur.LogoURL = o.LogoURL
	cur.City = o.City
	cur.Schedule = o.Schedule
	return nil
}

type stubAuditRepo struct{ logs []model.AuditLog }

func (s *stubAuditRepo) Create(_ context.Context, row *model.AuditLog) error {
	row.ID = uuid.New()
	s.logs = append(s.logs, *row)
	return nil
}

func TestOrgService_Create_RecordsAudit(t *testing.T) {
	orgs := newStubOrgRepo()
	audit := &stubAuditRepo{}
	svc := service.NewOrganizationService(orgs, audit)

	actor := uuid.New()
	o, err := svc.Create(context.Background(), actor, service.CreateOrgInput{
		Name: "Markaz X", Slug: "markaz-x", Country: "SO",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if orgs.created != o {
		t.Fatal("repo Create not called with returned org")
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "create_organization" {
		t.Fatalf("action=%s", audit.logs[0].Action)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("actor mismatch")
	}
}

func TestOrgService_Create_DefaultsTierAndStatus(t *testing.T) {
	orgs := newStubOrgRepo()
	svc := service.NewOrganizationService(orgs, &stubAuditRepo{})
	o, err := svc.Create(context.Background(), uuid.New(), service.CreateOrgInput{
		Name: "X", Slug: "x", Country: "SO",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if o.Tier != "free" {
		t.Fatalf("tier=%s want free", o.Tier)
	}
	if o.Status != "active" {
		t.Fatalf("status=%s want active", o.Status)
	}
}

func TestOrgService_GetBySlug(t *testing.T) {
	orgs := newStubOrgRepo()
	orgs.stored["foo"] = &model.Organization{ID: uuid.New(), Slug: "foo"}
	svc := service.NewOrganizationService(orgs, &stubAuditRepo{})
	got, err := svc.GetBySlug(context.Background(), "foo")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.Slug != "foo" {
		t.Fatalf("slug=%s", got.Slug)
	}
}

func TestOrgService_UpdateLanding_PersistsAndAudits(t *testing.T) {
	orgs := newStubOrgRepo()
	audit := &stubAuditRepo{}
	svc := service.NewOrganizationService(orgs, audit)

	o, err := svc.Create(context.Background(), uuid.New(), service.CreateOrgInput{
		Name: "Old", Slug: "u-1", Country: "SO",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	actor := uuid.New()
	desc := "New description"
	logo := "https://example.com/logo.png"
	city := "Mogadishu"
	schedule := []byte(`{"mon":"08:00-12:00"}`)
	in := service.UpdateLandingInput{
		Description: &desc,
		LogoURL:     &logo,
		City:        &city,
		Schedule:    schedule,
	}
	if err := svc.UpdateLanding(context.Background(), actor, o.ID, in); err != nil {
		t.Fatalf("UpdateLanding: %v", err)
	}

	got, err := orgs.GetByID(context.Background(), o.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Description == nil || *got.Description != desc {
		t.Fatalf("description: %v", got.Description)
	}
	if got.LogoURL == nil || *got.LogoURL != logo {
		t.Fatalf("logo: %v", got.LogoURL)
	}
	if got.City == nil || *got.City != city {
		t.Fatalf("city: %v", got.City)
	}
	if len(got.Schedule) == 0 {
		t.Fatalf("schedule not persisted")
	}

	if len(audit.logs) != 2 {
		t.Fatalf("audit logs=%d want 2 (create+update_landing)", len(audit.logs))
	}
	if audit.logs[1].Action != "update_landing" {
		t.Fatalf("audit action=%s want update_landing", audit.logs[1].Action)
	}
	if audit.logs[1].ActorID == nil || *audit.logs[1].ActorID != actor {
		t.Fatalf("audit actor mismatch")
	}
}

func TestOrgService_UpdateLanding_NotFound(t *testing.T) {
	orgs := newStubOrgRepo()
	svc := service.NewOrganizationService(orgs, &stubAuditRepo{})
	err := svc.UpdateLanding(context.Background(), uuid.New(), uuid.New(), service.UpdateLandingInput{})
	if err == nil {
		t.Fatalf("want error on missing org")
	}
}
