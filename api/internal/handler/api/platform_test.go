// api/internal/handler/api/platform_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubOrgService struct {
	createdInput service.CreateOrgInput
	createdActor uuid.UUID
	created      *model.Organization
	createErr    error
	listed       []model.Organization

	updateLandingActor uuid.UUID
	updateLandingOrgID uuid.UUID
	updateLandingInput service.UpdateLandingInput
	updateLandingErr   error
}

func (s *stubOrgService) Create(_ context.Context, actor uuid.UUID, in service.CreateOrgInput) (*model.Organization, error) {
	s.createdInput = in
	s.createdActor = actor
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.created = &model.Organization{ID: uuid.New(), Name: in.Name, Slug: in.Slug, Country: in.Country, Tier: "free", Status: "active"}
	return s.created, nil
}
func (s *stubOrgService) GetBySlug(_ context.Context, slug string) (*model.Organization, error) {
	return nil, nil
}
func (s *stubOrgService) List(_ context.Context, limit, offset int) ([]model.Organization, error) {
	return s.listed, nil
}
func (s *stubOrgService) UpdateLanding(_ context.Context, actor, orgID uuid.UUID, in service.UpdateLandingInput) error {
	s.updateLandingActor = actor
	s.updateLandingOrgID = orgID
	s.updateLandingInput = in
	return s.updateLandingErr
}

type stubInviteService struct {
	generated   *model.Invite
	gotActor    uuid.UUID
	gotOrgID    uuid.UUID
	generateErr error
}

func (s *stubInviteService) GenerateForCenterAdmin(_ context.Context, actor, orgID uuid.UUID) (*model.Invite, error) {
	s.gotActor = actor
	s.gotOrgID = orgID
	if s.generateErr != nil {
		return nil, s.generateErr
	}
	s.generated = &model.Invite{ID: uuid.New(), OrganizationID: orgID, Role: "center_admin", Token: "fake-token"}
	return s.generated, nil
}

func TestPlatform_CreateOrg_RequiresIdentity(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	body, _ := json.Marshal(map[string]string{"name": "X", "slug": "x", "country": "SO"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreateOrg(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without identity, got %d", rec.Code)
	}
}

func TestPlatform_CreateOrg_Success(t *testing.T) {
	orgs := &stubOrgService{}
	h := apihandler.NewPlatformHandler(orgs, &stubInviteService{})

	body, _ := json.Marshal(map[string]string{"name": "Markaz", "slug": "markaz", "country": "SO"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
	actor := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.CreateOrg(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if orgs.createdInput.Name != "Markaz" {
		t.Fatalf("name: %s", orgs.createdInput.Name)
	}
	if orgs.createdActor != actor {
		t.Fatalf("actor mismatch")
	}
}

func TestPlatform_CreateOrg_BadJSON(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("nope"))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.CreateOrg(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestPlatform_UpdateLanding_RequiresIdentity(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	body, _ := json.Marshal(map[string]any{"description": "x"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/organizations/me/landing", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.UpdateLanding(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestPlatform_UpdateLanding_RequiresOrgID(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	body, _ := json.Marshal(map[string]any{"description": "x"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/organizations/me/landing", bytes.NewReader(body))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.UpdateLanding(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestPlatform_UpdateLanding_HappyPath(t *testing.T) {
	orgs := &stubOrgService{}
	h := apihandler.NewPlatformHandler(orgs, &stubInviteService{})

	body, _ := json.Marshal(map[string]any{
		"description": "We are great",
		"logo_url":    "https://example.com/logo.png",
		"city":        "Mogadishu",
		"schedule":    map[string]string{"mon": "08:00-12:00"},
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/organizations/me/landing", bytes.NewReader(body))
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.UpdateLanding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if orgs.updateLandingActor != actor {
		t.Fatalf("actor mismatch")
	}
	if orgs.updateLandingOrgID != orgID {
		t.Fatalf("org mismatch")
	}
	if orgs.updateLandingInput.Description == nil || *orgs.updateLandingInput.Description != "We are great" {
		t.Fatalf("description: %+v", orgs.updateLandingInput)
	}
	if orgs.updateLandingInput.LogoURL == nil || *orgs.updateLandingInput.LogoURL != "https://example.com/logo.png" {
		t.Fatalf("logo: %+v", orgs.updateLandingInput)
	}
	if orgs.updateLandingInput.City == nil || *orgs.updateLandingInput.City != "Mogadishu" {
		t.Fatalf("city: %+v", orgs.updateLandingInput)
	}
	if len(orgs.updateLandingInput.Schedule) == 0 {
		t.Fatalf("schedule not forwarded")
	}
}

func TestPlatform_UpdateLanding_BadJSON(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/organizations/me/landing", bytes.NewReader([]byte("not-json")))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.UpdateLanding(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestPlatform_GenerateInvite_Success(t *testing.T) {
	invs := &stubInviteService{}
	h := apihandler.NewPlatformHandler(&stubOrgService{}, invs)

	orgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/invite", nil)
	apihandler.SetURLParamForTest(req, "id", orgID.String())
	actor := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.GenerateInvite(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if invs.gotActor != actor {
		t.Fatalf("actor mismatch")
	}
	if invs.gotOrgID != orgID {
		t.Fatalf("org_id mismatch")
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data.Token == "" {
		t.Fatal("token empty in response")
	}
}
