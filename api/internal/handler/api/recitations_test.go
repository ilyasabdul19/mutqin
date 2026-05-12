// api/internal/handler/api/recitations_test.go
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
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubRecitationService struct {
	recordActor    uuid.UUID
	recordOrg      uuid.UUID
	recordInput    service.RecordInput
	recordReturn   *model.Recitation
	recordErr      error

	batchActor   uuid.UUID
	batchOrg     uuid.UUID
	batchInputs  []service.RecordInput
	batchReturn  []model.Recitation
	batchErr     error

	listGotStudent uuid.UUID
	listLimit      int
	listOffset     int
	listReturn     []model.Recitation
	listErr        error

	latestGotStudent uuid.UUID
	latestReturn     *model.Recitation
	latestErr        error
}

func (s *stubRecitationService) Record(_ context.Context, actor, orgID uuid.UUID, in service.RecordInput) (*model.Recitation, error) {
	s.recordActor = actor
	s.recordOrg = orgID
	s.recordInput = in
	if s.recordErr != nil {
		return nil, s.recordErr
	}
	if s.recordReturn != nil {
		return s.recordReturn, nil
	}
	return &model.Recitation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		TeacherID:      actor,
		StudentID:      in.StudentID,
		HalaqahID:      in.HalaqahID,
		Type:           in.Type,
		SurahNumber:    in.SurahNumber,
		AyahFrom:       in.AyahFrom,
		AyahTo:         in.AyahTo,
		Grade:          in.Grade,
		Notes:          in.Notes,
		ClientID:       in.ClientID,
	}, nil
}

func (s *stubRecitationService) RecordBatch(_ context.Context, actor, orgID uuid.UUID, ins []service.RecordInput) ([]model.Recitation, error) {
	s.batchActor = actor
	s.batchOrg = orgID
	s.batchInputs = ins
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	if s.batchReturn != nil {
		return s.batchReturn, nil
	}
	out := make([]model.Recitation, len(ins))
	for i, in := range ins {
		out[i] = model.Recitation{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TeacherID:      actor,
			StudentID:      in.StudentID,
			HalaqahID:      in.HalaqahID,
			Type:           in.Type,
			SurahNumber:    in.SurahNumber,
			AyahFrom:       in.AyahFrom,
			AyahTo:         in.AyahTo,
			Grade:          in.Grade,
		}
	}
	return out, nil
}

func (s *stubRecitationService) ListForStudent(_ context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error) {
	s.listGotStudent = studentID
	s.listLimit = limit
	s.listOffset = offset
	return s.listReturn, s.listErr
}

func (s *stubRecitationService) GetLatest(_ context.Context, studentID uuid.UUID) (*model.Recitation, error) {
	s.latestGotStudent = studentID
	if s.latestErr != nil {
		return nil, s.latestErr
	}
	return s.latestReturn, nil
}

func validRecordBody(studentID, halaqahID uuid.UUID) map[string]any {
	return map[string]any{
		"student_id":   studentID.String(),
		"halaqah_id":   halaqahID.String(),
		"type":         "new_hifz",
		"surah_number": 2,
		"ayah_from":    1,
		"ayah_to":      5,
		"grade":        "mumtaz",
	}
}

func TestRecitations_Record_RequiresIdentity(t *testing.T) {
	h := apihandler.NewRecitationsHandler(&stubRecitationService{})
	body, _ := json.Marshal(validRecordBody(uuid.New(), uuid.New()))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Record(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestRecitations_Record_RequiresOrgID(t *testing.T) {
	h := apihandler.NewRecitationsHandler(&stubRecitationService{})
	body, _ := json.Marshal(validRecordBody(uuid.New(), uuid.New()))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations", bytes.NewReader(body))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.Record(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestRecitations_Record_HappyPath(t *testing.T) {
	svc := &stubRecitationService{}
	h := apihandler.NewRecitationsHandler(svc)

	studentID := uuid.New()
	halaqahID := uuid.New()
	b := validRecordBody(studentID, halaqahID)
	b["notes"] = "good"
	b["client_id"] = "client-abc"
	body, _ := json.Marshal(b)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations", bytes.NewReader(body))
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Record(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.recordActor != actor {
		t.Fatalf("actor=%s want %s", svc.recordActor, actor)
	}
	if svc.recordOrg != orgID {
		t.Fatalf("org mismatch")
	}
	if svc.recordInput.StudentID != studentID || svc.recordInput.HalaqahID != halaqahID {
		t.Fatalf("ids mismatch")
	}
	if svc.recordInput.Type != "new_hifz" {
		t.Fatalf("type=%s", svc.recordInput.Type)
	}
	if svc.recordInput.Grade != "mumtaz" {
		t.Fatalf("grade=%s", svc.recordInput.Grade)
	}
	if svc.recordInput.SurahNumber != 2 || svc.recordInput.AyahFrom != 1 || svc.recordInput.AyahTo != 5 {
		t.Fatalf("ayah range mismatch")
	}
	if svc.recordInput.Notes == nil || *svc.recordInput.Notes != "good" {
		t.Fatalf("notes=%v", svc.recordInput.Notes)
	}
	if svc.recordInput.ClientID == nil || *svc.recordInput.ClientID != "client-abc" {
		t.Fatalf("client_id=%v", svc.recordInput.ClientID)
	}
}

func TestRecitations_Record_InvalidInputIs400(t *testing.T) {
	svc := &stubRecitationService{recordErr: service.ErrInvalidRecitationInput}
	h := apihandler.NewRecitationsHandler(svc)
	body, _ := json.Marshal(validRecordBody(uuid.New(), uuid.New()))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations", bytes.NewReader(body))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Record(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecitations_Record_BadJSONIs400(t *testing.T) {
	h := apihandler.NewRecitationsHandler(&stubRecitationService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations", bytes.NewReader([]byte("{ not json")))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Record(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestRecitations_RecordBatch_HappyPath(t *testing.T) {
	svc := &stubRecitationService{}
	h := apihandler.NewRecitationsHandler(svc)

	studentID := uuid.New()
	halaqahID := uuid.New()
	items := []map[string]any{
		validRecordBody(studentID, halaqahID),
		validRecordBody(studentID, halaqahID),
		validRecordBody(studentID, halaqahID),
	}
	body, _ := json.Marshal(items)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations/batch", bytes.NewReader(body))
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.RecordBatch(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.batchActor != actor || svc.batchOrg != orgID {
		t.Fatalf("actor/org mismatch")
	}
	if len(svc.batchInputs) != 3 {
		t.Fatalf("batch inputs=%d want 3", len(svc.batchInputs))
	}
	for i, in := range svc.batchInputs {
		if in.StudentID != studentID || in.HalaqahID != halaqahID {
			t.Fatalf("idx=%d ids mismatch", i)
		}
		if in.Type != "new_hifz" {
			t.Fatalf("idx=%d type=%s", i, in.Type)
		}
	}
}

func TestRecitations_RecordBatch_InvalidInputIs400(t *testing.T) {
	svc := &stubRecitationService{batchErr: service.ErrInvalidRecitationInput}
	h := apihandler.NewRecitationsHandler(svc)
	body, _ := json.Marshal([]map[string]any{validRecordBody(uuid.New(), uuid.New())})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations/batch", bytes.NewReader(body))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.RecordBatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestRecitations_RecordBatch_RequiresOrgID(t *testing.T) {
	h := apihandler.NewRecitationsHandler(&stubRecitationService{})
	body, _ := json.Marshal([]map[string]any{validRecordBody(uuid.New(), uuid.New())})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recitations/batch", bytes.NewReader(body))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.RecordBatch(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestRecitations_ListByStudent_PassThrough(t *testing.T) {
	svc := &stubRecitationService{listReturn: []model.Recitation{
		{ID: uuid.New(), Grade: "mumtaz"},
		{ID: uuid.New(), Grade: "jayyid"},
	}}
	h := apihandler.NewRecitationsHandler(svc)

	studentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/"+studentID.String()+"/recitations?limit=20&offset=5", nil)
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.ListByStudent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.listGotStudent != studentID {
		t.Fatalf("student id mismatch")
	}
	if svc.listLimit != 20 || svc.listOffset != 5 {
		t.Fatalf("limit=%d offset=%d", svc.listLimit, svc.listOffset)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("len=%d", len(resp.Data))
	}
}

func TestRecitations_ListByStudent_RequiresIdentity(t *testing.T) {
	h := apihandler.NewRecitationsHandler(&stubRecitationService{})
	studentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/"+studentID.String()+"/recitations", nil)
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	rec := httptest.NewRecorder()
	h.ListByStudent(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestRecitations_LatestByStudent_HappyPath(t *testing.T) {
	studentID := uuid.New()
	svc := &stubRecitationService{
		latestReturn: &model.Recitation{ID: uuid.New(), StudentID: studentID, Grade: "mumtaz"},
	}
	h := apihandler.NewRecitationsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/"+studentID.String()+"/recitations/latest", nil)
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.LatestByStudent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.latestGotStudent != studentID {
		t.Fatalf("student id mismatch")
	}
}

func TestRecitations_LatestByStudent_NotFoundIs404(t *testing.T) {
	svc := &stubRecitationService{latestErr: repo.ErrNotFound}
	h := apihandler.NewRecitationsHandler(svc)

	studentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/"+studentID.String()+"/recitations/latest", nil)
	apihandler.SetURLParamForTest(req, "id", studentID.String())
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.LatestByStudent(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecitations_LatestByStudent_BadID(t *testing.T) {
	h := apihandler.NewRecitationsHandler(&stubRecitationService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/bad/recitations/latest", nil)
	apihandler.SetURLParamForTest(req, "id", "bad")
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.LatestByStudent(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}
