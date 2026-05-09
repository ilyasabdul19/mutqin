// api/internal/handler/api/students_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubStudentService struct {
	enrollActor    uuid.UUID
	enrollOrg      uuid.UUID
	enrollHalaqah  uuid.UUID
	enrollInput    service.EnrollStudentInput
	enrollReturn   *model.Student
	enrollErr      error

	listGotHalaqah uuid.UUID
	listLimit      int
	listOffset     int
	listReturn     []model.Student
	listErr        error

	transferActor   uuid.UUID
	transferStudent uuid.UUID
	transferNew     uuid.UUID
	transferOrg     uuid.UUID
	transferErr     error
}

func (s *stubStudentService) Enroll(_ context.Context, actor, orgID, halaqahID uuid.UUID, in service.EnrollStudentInput) (*model.Student, error) {
	s.enrollActor = actor
	s.enrollOrg = orgID
	s.enrollHalaqah = halaqahID
	s.enrollInput = in
	if s.enrollErr != nil {
		return nil, s.enrollErr
	}
	if s.enrollReturn != nil {
		return s.enrollReturn, nil
	}
	return &model.Student{
		ID:             uuid.New(),
		OrganizationID: orgID,
		HalaqahID:      &halaqahID,
		Name:           in.Name,
		Status:         "active",
	}, nil
}

func (s *stubStudentService) ListByHalaqah(_ context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error) {
	s.listGotHalaqah = halaqahID
	s.listLimit = limit
	s.listOffset = offset
	return s.listReturn, s.listErr
}

func (s *stubStudentService) Transfer(_ context.Context, actor, studentID, newHalaqahID, orgID uuid.UUID) error {
	s.transferActor = actor
	s.transferStudent = studentID
	s.transferNew = newHalaqahID
	s.transferOrg = orgID
	return s.transferErr
}

func TestStudents_Enroll_RequiresIdentity(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	body, _ := json.Marshal(map[string]any{"name": "Ali"})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/"+halaqahID.String()+"/students", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	rec := httptest.NewRecorder()
	h.Enroll(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestStudents_Enroll_RequiresOrgID(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	body, _ := json.Marshal(map[string]any{"name": "Ali"})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/"+halaqahID.String()+"/students", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.Enroll(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestStudents_Enroll_HappyPath(t *testing.T) {
	svc := &stubStudentService{}
	h := apihandler.NewStudentsHandler(svc)

	age := 12
	parentEmail := "parent@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":         "Ali",
		"age":          age,
		"parent_email": parentEmail,
	})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/"+halaqahID.String()+"/students", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Enroll(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.enrollActor != actor {
		t.Fatal("actor mismatch")
	}
	if svc.enrollOrg != orgID {
		t.Fatal("org mismatch")
	}
	if svc.enrollHalaqah != halaqahID {
		t.Fatal("halaqah mismatch")
	}
	if svc.enrollInput.Name != "Ali" {
		t.Fatalf("name=%s", svc.enrollInput.Name)
	}
	if svc.enrollInput.Age == nil || *svc.enrollInput.Age != 12 {
		t.Fatalf("age=%v", svc.enrollInput.Age)
	}
	if svc.enrollInput.ParentEmail == nil || *svc.enrollInput.ParentEmail != parentEmail {
		t.Fatalf("parent_email=%v", svc.enrollInput.ParentEmail)
	}
}

func TestStudents_Enroll_BadHalaqahID(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	body, _ := json.Marshal(map[string]any{"name": "Ali"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/bad/students", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", "bad")
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Enroll(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestStudents_Enroll_NameRequired(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	body, _ := json.Marshal(map[string]any{"name": "  "})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/"+halaqahID.String()+"/students", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Enroll(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestStudents_Enroll_CrossTenantIs403(t *testing.T) {
	svc := &stubStudentService{enrollErr: service.ErrCrossTenant}
	h := apihandler.NewStudentsHandler(svc)
	body, _ := json.Marshal(map[string]any{"name": "Ali"})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/"+halaqahID.String()+"/students", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Enroll(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d", rec.Code)
	}
}

func TestStudents_ListByHalaqah_PassThrough(t *testing.T) {
	svc := &stubStudentService{listReturn: []model.Student{
		{ID: uuid.New(), Name: "Ali"},
		{ID: uuid.New(), Name: "Omar"},
	}}
	h := apihandler.NewStudentsHandler(svc)
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat/"+halaqahID.String()+"/students?limit=20&offset=5", nil)
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.ListByHalaqah(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.listGotHalaqah != halaqahID {
		t.Fatal("halaqah id mismatch")
	}
	if svc.listLimit != 20 {
		t.Fatalf("limit=%d", svc.listLimit)
	}
	if svc.listOffset != 5 {
		t.Fatalf("offset=%d", svc.listOffset)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("len=%d", len(resp.Data))
	}
}

func TestStudents_ListByHalaqah_RequiresIdentity(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat/"+halaqahID.String()+"/students", nil)
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	rec := httptest.NewRecorder()
	h.ListByHalaqah(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestStudents_Transfer_HappyPath(t *testing.T) {
	svc := &stubStudentService{}
	h := apihandler.NewStudentsHandler(svc)

	studentID := uuid.New()
	newHalaqahID := uuid.New()
	body, _ := json.Marshal(map[string]any{"new_halaqah_id": newHalaqahID.String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/students/"+studentID.String()+"/transfer", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Transfer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.transferActor != actor || svc.transferStudent != studentID || svc.transferNew != newHalaqahID || svc.transferOrg != orgID {
		t.Fatalf("transfer args mismatch: actor=%s student=%s new=%s org=%s",
			svc.transferActor, svc.transferStudent, svc.transferNew, svc.transferOrg)
	}
	var resp struct {
		Data map[string]string `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data["status"] != "transferred" {
		t.Fatalf("status=%s", resp.Data["status"])
	}
}

func TestStudents_Transfer_CrossTenant403(t *testing.T) {
	svc := &stubStudentService{transferErr: service.ErrCrossTenant}
	h := apihandler.NewStudentsHandler(svc)

	studentID := uuid.New()
	newHalaqahID := uuid.New()
	body, _ := json.Marshal(map[string]any{"new_halaqah_id": newHalaqahID.String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/students/"+studentID.String()+"/transfer", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Transfer(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStudents_Transfer_BadStudentID(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	body, _ := json.Marshal(map[string]any{"new_halaqah_id": uuid.New().String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/students/bad/transfer", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", "bad")
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Transfer(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestStudents_Transfer_BadNewHalaqahID(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	studentID := uuid.New()
	body, _ := json.Marshal(map[string]any{"new_halaqah_id": "bad"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/students/"+studentID.String()+"/transfer", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.Transfer(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestStudents_Transfer_RequiresIdentity(t *testing.T) {
	h := apihandler.NewStudentsHandler(&stubStudentService{})
	studentID := uuid.New()
	body, _ := json.Marshal(map[string]any{"new_halaqah_id": uuid.New().String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/students/"+studentID.String()+"/transfer", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	rec := httptest.NewRecorder()
	h.Transfer(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}
