// api/internal/handler/api/halaqat_test.go
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

type stubHalaqatService struct {
	createdActor uuid.UUID
	createdOrg   uuid.UUID
	createdInput service.CreateHalaqahInput
	createReturn *model.Halaqah
	createErr    error

	getReturn *model.Halaqah
	getErr    error

	listReturn []model.Halaqah
	listErr    error

	updateReturn *model.Halaqah
	updatedActor uuid.UUID
	updateErr    error
}

func (s *stubHalaqatService) Create(_ context.Context, actor, orgID uuid.UUID, in service.CreateHalaqahInput) (*model.Halaqah, error) {
	s.createdActor = actor
	s.createdOrg = orgID
	s.createdInput = in
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createReturn != nil {
		return s.createReturn, nil
	}
	cap := in.MaxCapacity
	if cap <= 0 {
		cap = 30
	}
	return &model.Halaqah{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           in.Name,
		MaxCapacity:    cap,
		TeacherID:      in.TeacherID,
		Status:         "active",
	}, nil
}

func (s *stubHalaqatService) GetByID(_ context.Context, id uuid.UUID) (*model.Halaqah, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getReturn != nil {
		return s.getReturn, nil
	}
	return &model.Halaqah{ID: id, Name: "stub"}, nil
}

func (s *stubHalaqatService) List(_ context.Context, _, _ int) ([]model.Halaqah, error) {
	return s.listReturn, s.listErr
}

func (s *stubHalaqatService) Update(_ context.Context, actor uuid.UUID, h *model.Halaqah) error {
	s.updatedActor = actor
	s.updateReturn = h
	return s.updateErr
}

func TestHalaqat_Create_RequiresIdentity(t *testing.T) {
	h := apihandler.NewHalaqatHandler(&stubHalaqatService{})
	body, _ := json.Marshal(map[string]any{"name": "Al-Fajr", "max_capacity": 25})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without identity, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHalaqat_Create_RequiresOrgID(t *testing.T) {
	h := apihandler.NewHalaqatHandler(&stubHalaqatService{})
	body, _ := json.Marshal(map[string]any{"name": "Al-Fajr", "max_capacity": 25})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat", bytes.NewReader(body))
	// super_admin without OrgID — must be rejected
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 without OrgID, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHalaqat_Create_Success(t *testing.T) {
	svc := &stubHalaqatService{}
	h := apihandler.NewHalaqatHandler(svc)

	body, _ := json.Marshal(map[string]any{"name": "Al-Asr", "max_capacity": 25})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat", bytes.NewReader(body))
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createdActor != actor {
		t.Fatalf("actor mismatch")
	}
	if svc.createdOrg != orgID {
		t.Fatalf("org id mismatch")
	}
	if svc.createdInput.Name != "Al-Asr" {
		t.Fatalf("name: %s", svc.createdInput.Name)
	}
	if svc.createdInput.MaxCapacity != 25 {
		t.Fatalf("max_capacity: %d", svc.createdInput.MaxCapacity)
	}
}

func TestHalaqat_Create_BadJSON(t *testing.T) {
	h := apihandler.NewHalaqatHandler(&stubHalaqatService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat", bytes.NewReader([]byte("not-json")))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestHalaqat_Create_NameRequired(t *testing.T) {
	h := apihandler.NewHalaqatHandler(&stubHalaqatService{})
	body, _ := json.Marshal(map[string]any{"name": "  "})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat", bytes.NewReader(body))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestHalaqat_Create_DefaultsCapacityTo30(t *testing.T) {
	svc := &stubHalaqatService{}
	h := apihandler.NewHalaqatHandler(svc)

	body, _ := json.Marshal(map[string]any{"name": "Al-Maghrib"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat", bytes.NewReader(body))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createdInput.MaxCapacity != 30 {
		t.Fatalf("default capacity: got %d want 30", svc.createdInput.MaxCapacity)
	}
}

func TestHalaqat_Get_404(t *testing.T) {
	svc := &stubHalaqatService{getErr: repo.ErrNotFound}
	h := apihandler.NewHalaqatHandler(svc)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat/"+id.String(), nil)
	apihandler.SetURLParamForTest(req, "id", id.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rec.Code)
	}
}

func TestHalaqat_Get_BadID(t *testing.T) {
	h := apihandler.NewHalaqatHandler(&stubHalaqatService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat/not-a-uuid", nil)
	apihandler.SetURLParamForTest(req, "id", "not-a-uuid")
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestHalaqat_List_Success(t *testing.T) {
	svc := &stubHalaqatService{listReturn: []model.Halaqah{
		{ID: uuid.New(), Name: "A"},
		{ID: uuid.New(), Name: "B"},
	}}
	h := apihandler.NewHalaqatHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat?limit=10&offset=0", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]int   `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len=%d want 2", len(resp.Data))
	}
}

func TestHalaqat_List_RequiresIdentity(t *testing.T) {
	h := apihandler.NewHalaqatHandler(&stubHalaqatService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestHalaqat_Update_Success(t *testing.T) {
	existing := &model.Halaqah{ID: uuid.New(), Name: "Old", MaxCapacity: 20, Status: "active"}
	svc := &stubHalaqatService{getReturn: existing}
	h := apihandler.NewHalaqatHandler(svc)

	teacherID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"name":         "New Name",
		"max_capacity": 35,
		"teacher_id":   teacherID.String(),
		"status":       "inactive",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/halaqat/"+existing.ID.String(), bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", existing.ID.String())
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.updatedActor != actor {
		t.Fatalf("actor mismatch")
	}
	if svc.updateReturn == nil {
		t.Fatal("update not called")
	}
	if svc.updateReturn.Name != "New Name" {
		t.Fatalf("name=%s", svc.updateReturn.Name)
	}
	if svc.updateReturn.MaxCapacity != 35 {
		t.Fatalf("cap=%d", svc.updateReturn.MaxCapacity)
	}
	if svc.updateReturn.TeacherID == nil || *svc.updateReturn.TeacherID != teacherID {
		t.Fatalf("teacher_id mismatch")
	}
	if svc.updateReturn.Status != "inactive" {
		t.Fatalf("status=%s", svc.updateReturn.Status)
	}
}

func TestHalaqat_Update_NotFound(t *testing.T) {
	svc := &stubHalaqatService{getErr: repo.ErrNotFound}
	h := apihandler.NewHalaqatHandler(svc)

	id := uuid.New()
	body, _ := json.Marshal(map[string]any{"name": "X"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/halaqat/"+id.String(), bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", id.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rec.Code)
	}
}

func TestHalaqat_Update_ServiceError(t *testing.T) {
	existing := &model.Halaqah{ID: uuid.New(), Name: "Old"}
	svc := &stubHalaqatService{getReturn: existing, updateErr: errors.New("boom")}
	h := apihandler.NewHalaqatHandler(svc)

	body, _ := json.Marshal(map[string]any{"name": "Y"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/halaqat/"+existing.ID.String(), bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", existing.ID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}
