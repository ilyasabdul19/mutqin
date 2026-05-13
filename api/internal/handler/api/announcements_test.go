// api/internal/handler/api/announcements_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubAnnouncementService struct {
	createActor   uuid.UUID
	createOrg     uuid.UUID
	createInput   service.CreateAnnouncementInput
	createReturn  *model.Announcement
	createErr     error

	listLimit  int
	listOffset int
	listReturn []model.Announcement
	listErr    error

	deleteActor uuid.UUID
	deleteID    uuid.UUID
	deleteErr   error
}

func (s *stubAnnouncementService) Create(_ context.Context, actor, orgID uuid.UUID, in service.CreateAnnouncementInput) (*model.Announcement, error) {
	s.createActor = actor
	s.createOrg = orgID
	s.createInput = in
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createReturn != nil {
		return s.createReturn, nil
	}
	return &model.Announcement{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Title:          in.Title,
		Body:           in.Body,
	}, nil
}

func (s *stubAnnouncementService) ListByOrg(_ context.Context, limit, offset int) ([]model.Announcement, error) {
	s.listLimit = limit
	s.listOffset = offset
	return s.listReturn, s.listErr
}

func (s *stubAnnouncementService) Delete(_ context.Context, actor, id uuid.UUID) error {
	s.deleteActor = actor
	s.deleteID = id
	return s.deleteErr
}

func TestAnnouncements_Create_RequiresIdentity(t *testing.T) {
	h := apihandler.NewAnnouncementsHandler(&stubAnnouncementService{})
	body, _ := json.Marshal(map[string]any{"title": "x", "body": "y"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestAnnouncements_Create_RequiresOrgID(t *testing.T) {
	h := apihandler.NewAnnouncementsHandler(&stubAnnouncementService{})
	body, _ := json.Marshal(map[string]any{"title": "x", "body": "y"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader(body))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAnnouncements_Create_HappyPath(t *testing.T) {
	svc := &stubAnnouncementService{}
	h := apihandler.NewAnnouncementsHandler(svc)

	body, _ := json.Marshal(map[string]any{
		"title": "Eid",
		"body":  "Classes closed Friday.",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader(body))
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createActor != actor || svc.createOrg != orgID {
		t.Fatalf("actor/org mismatch")
	}
	if svc.createInput.Title != "Eid" || svc.createInput.Body != "Classes closed Friday." {
		t.Fatalf("input mismatch: %+v", svc.createInput)
	}
}

func TestAnnouncements_Create_ValidationMapsTo400(t *testing.T) {
	svc := &stubAnnouncementService{createErr: service.ErrInvalidInput}
	h := apihandler.NewAnnouncementsHandler(svc)

	body, _ := json.Marshal(map[string]any{"title": "x", "body": "y"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader(body))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAnnouncements_Create_BadJSON(t *testing.T) {
	h := apihandler.NewAnnouncementsHandler(&stubAnnouncementService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/announcements", bytes.NewReader([]byte("nope")))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAnnouncements_List_RequiresIdentity(t *testing.T) {
	h := apihandler.NewAnnouncementsHandler(&stubAnnouncementService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestAnnouncements_List_PassThrough(t *testing.T) {
	svc := &stubAnnouncementService{listReturn: []model.Announcement{
		{ID: uuid.New(), Title: "a"},
		{ID: uuid.New(), Title: "b"},
	}}
	h := apihandler.NewAnnouncementsHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/announcements?limit=20&offset=3", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.listLimit != 20 || svc.listOffset != 3 {
		t.Fatalf("limit/offset = %d/%d", svc.listLimit, svc.listOffset)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("len=%d", len(resp.Data))
	}
}

func TestAnnouncements_Delete_RequiresIdentity(t *testing.T) {
	h := apihandler.NewAnnouncementsHandler(&stubAnnouncementService{})
	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/announcements/"+id.String(), nil)
	apihandler.SetURLParamForTest(req, "id", id.String())
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestAnnouncements_Delete_BadID(t *testing.T) {
	h := apihandler.NewAnnouncementsHandler(&stubAnnouncementService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/announcements/bad", nil)
	apihandler.SetURLParamForTest(req, "id", "bad")
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAnnouncements_Delete_HappyPath(t *testing.T) {
	svc := &stubAnnouncementService{}
	h := apihandler.NewAnnouncementsHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/announcements/"+id.String(), nil)
	apihandler.SetURLParamForTest(req, "id", id.String())
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204 got %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.deleteActor != actor || svc.deleteID != id {
		t.Fatalf("delete args mismatch")
	}
}

func TestAnnouncements_Delete_NotFound(t *testing.T) {
	svc := &stubAnnouncementService{deleteErr: repo.ErrNotFound}
	h := apihandler.NewAnnouncementsHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/announcements/"+id.String(), nil)
	apihandler.SetURLParamForTest(req, "id", id.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rec.Code)
	}
}

// guard against accidental drift in the error-wrap chain
var _ = errors.Is
