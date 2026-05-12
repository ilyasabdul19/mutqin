// api/internal/handler/api/attendance_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubAttendanceService struct {
	markActor     uuid.UUID
	markOrg       uuid.UUID
	markHalaqah   uuid.UUID
	markDate      time.Time
	markMarks     []service.MarkInput
	markReturn    []model.Attendance
	markErr       error

	listHalaqahReturn []model.Attendance
	listHalaqahErr    error
	lastListHalaqah   uuid.UUID
	lastListDate      time.Time

	listStudentReturn []model.Attendance
	listStudentErr    error
	lastListStudent   uuid.UUID
	lastListStuFrom   time.Time
	lastListStuTo     time.Time
	lastListStuLimit  int

	listOrgReturn  []model.Attendance
	listOrgErr     error
	lastListOrgH   *uuid.UUID
	lastListOrgLim int
}

func (s *stubAttendanceService) MarkBatch(_ context.Context, actor, orgID, halaqahID uuid.UUID, date time.Time, marks []service.MarkInput) ([]model.Attendance, error) {
	s.markActor = actor
	s.markOrg = orgID
	s.markHalaqah = halaqahID
	s.markDate = date
	s.markMarks = marks
	if s.markErr != nil {
		return nil, s.markErr
	}
	if s.markReturn != nil {
		return s.markReturn, nil
	}
	rows := make([]model.Attendance, 0, len(marks))
	for _, m := range marks {
		rows = append(rows, model.Attendance{
			ID:             uuid.New(),
			OrganizationID: orgID,
			HalaqahID:      halaqahID,
			StudentID:      m.StudentID,
			Date:           date,
			Status:         m.Status,
		})
	}
	return rows, nil
}

func (s *stubAttendanceService) ListByHalaqahDate(_ context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error) {
	s.lastListHalaqah = halaqahID
	s.lastListDate = date
	return s.listHalaqahReturn, s.listHalaqahErr
}

func (s *stubAttendanceService) ListByStudent(_ context.Context, stuID uuid.UUID, from, to time.Time, limit, _ int) ([]model.Attendance, error) {
	s.lastListStudent = stuID
	s.lastListStuFrom = from
	s.lastListStuTo = to
	s.lastListStuLimit = limit
	return s.listStudentReturn, s.listStudentErr
}

func (s *stubAttendanceService) ListByOrg(_ context.Context, halaqahID *uuid.UUID, _, _ time.Time, limit, _ int) ([]model.Attendance, error) {
	s.lastListOrgH = halaqahID
	s.lastListOrgLim = limit
	return s.listOrgReturn, s.listOrgErr
}

func authedReq(req *http.Request, role string) *http.Request {
	orgID := uuid.New()
	return req.WithContext(auth.With(req.Context(), auth.Identity{
		UserID: uuid.New(),
		OrgID:  &orgID,
		Role:   role,
	}))
}

func TestAttendance_MarkBatch_Success(t *testing.T) {
	svc := &stubAttendanceService{}
	h := apihandler.NewAttendanceHandler(svc)

	halaqahID := uuid.New()
	stu1 := uuid.New()
	stu2 := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"date": "2026-05-12",
		"marks": []map[string]any{
			{"student_id": stu1.String(), "status": "present"},
			{"student_id": stu2.String(), "status": "absent"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/halaqat/%s/attendance", halaqahID), bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	req = authedReq(req, "teacher")

	rec := httptest.NewRecorder()
	h.MarkBatch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.markHalaqah != halaqahID {
		t.Fatalf("halaqah mismatch")
	}
	if len(svc.markMarks) != 2 {
		t.Fatalf("marks=%d want 2", len(svc.markMarks))
	}
	if svc.markMarks[0].StudentID != stu1 || svc.markMarks[0].Status != "present" {
		t.Fatalf("mark[0] mismatch")
	}
	if svc.markDate.Format("2006-01-02") != "2026-05-12" {
		t.Fatalf("date=%v", svc.markDate)
	}

	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]int   `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Data) != 2 {
		t.Fatalf("rows=%d want 2", len(resp.Data))
	}
}

func TestAttendance_MarkBatch_RequiresAuth(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	body, _ := json.Marshal(map[string]any{
		"date":  "2026-05-12",
		"marks": []map[string]any{{"student_id": uuid.New().String(), "status": "present"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/xx/attendance", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", uuid.New().String())
	rec := httptest.NewRecorder()
	h.MarkBatch(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestAttendance_MarkBatch_BadDate(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	body, _ := json.Marshal(map[string]any{
		"date":  "12-05-2026",
		"marks": []map[string]any{{"student_id": uuid.New().String(), "status": "present"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/x/attendance", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", uuid.New().String())
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.MarkBatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAttendance_MarkBatch_BadHalaqahID(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	body, _ := json.Marshal(map[string]any{
		"date":  "2026-05-12",
		"marks": []map[string]any{{"student_id": uuid.New().String(), "status": "present"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/halaqat/notuuid/attendance", bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", "notuuid")
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.MarkBatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAttendance_MarkBatch_InvalidStatus_400(t *testing.T) {
	svc := &stubAttendanceService{markErr: fmt.Errorf("wrap: %w", service.ErrInvalidAttendance)}
	h := apihandler.NewAttendanceHandler(svc)

	halaqahID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"date":  "2026-05-12",
		"marks": []map[string]any{{"student_id": uuid.New().String(), "status": "maybe"}},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/halaqat/%s/attendance", halaqahID), bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.MarkBatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAttendance_MarkBatch_ServiceFails_500(t *testing.T) {
	svc := &stubAttendanceService{markErr: errors.New("boom")}
	h := apihandler.NewAttendanceHandler(svc)
	halaqahID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"date":  "2026-05-12",
		"marks": []map[string]any{{"student_id": uuid.New().String(), "status": "present"}},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/halaqat/%s/attendance", halaqahID), bytes.NewReader(body))
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.MarkBatch(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}

func TestAttendance_ListByHalaqahDate_Success(t *testing.T) {
	svc := &stubAttendanceService{listHalaqahReturn: []model.Attendance{
		{ID: uuid.New(), Status: "present"},
		{ID: uuid.New(), Status: "absent"},
	}}
	h := apihandler.NewAttendanceHandler(svc)

	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/halaqat/%s/attendance?date=2026-05-12", halaqahID), nil)
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.ListByHalaqahDate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListHalaqah != halaqahID {
		t.Fatalf("halaqah mismatch")
	}
	if svc.lastListDate.Format("2006-01-02") != "2026-05-12" {
		t.Fatalf("date=%v", svc.lastListDate)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Fatalf("rows=%d want 2", len(resp.Data))
	}
}

func TestAttendance_ListByHalaqahDate_BadDate_400(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	halaqahID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/halaqat/%s/attendance?date=nope", halaqahID), nil)
	apihandler.SetURLParamForTest(req, "id", halaqahID.String())
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.ListByHalaqahDate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAttendance_ListByHalaqahDate_BadHalaqahID(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/halaqat/nope/attendance?date=2026-05-12", nil)
	apihandler.SetURLParamForTest(req, "id", "nope")
	req = authedReq(req, "teacher")
	rec := httptest.NewRecorder()
	h.ListByHalaqahDate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAttendance_ListByStudent_Success_DefaultsRange(t *testing.T) {
	svc := &stubAttendanceService{listStudentReturn: []model.Attendance{
		{ID: uuid.New(), Status: "present"},
	}}
	h := apihandler.NewAttendanceHandler(svc)
	stuID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/students/%s/attendance", stuID), nil)
	apihandler.SetURLParamForTest(req, "id", stuID.String())
	req = authedReq(req, "center_admin")
	rec := httptest.NewRecorder()
	h.ListByStudent(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListStudent != stuID {
		t.Fatalf("student mismatch")
	}
	// Defaults: from = ~30 days ago, to = today.
	if svc.lastListStuTo.IsZero() || svc.lastListStuFrom.IsZero() {
		t.Fatalf("from/to zero — defaults not applied")
	}
}

func TestAttendance_ListByStudent_ExplicitRange(t *testing.T) {
	svc := &stubAttendanceService{}
	h := apihandler.NewAttendanceHandler(svc)
	stuID := uuid.New()
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/students/%s/attendance?from=2026-05-01&to=2026-05-10&limit=20&offset=5", stuID), nil)
	apihandler.SetURLParamForTest(req, "id", stuID.String())
	req = authedReq(req, "center_admin")
	rec := httptest.NewRecorder()
	h.ListByStudent(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListStuFrom.Format("2006-01-02") != "2026-05-01" {
		t.Fatalf("from=%v", svc.lastListStuFrom)
	}
	if svc.lastListStuTo.Format("2006-01-02") != "2026-05-10" {
		t.Fatalf("to=%v", svc.lastListStuTo)
	}
	if svc.lastListStuLimit != 20 {
		t.Fatalf("limit=%d want 20", svc.lastListStuLimit)
	}
}

func TestAttendance_ListByStudent_BadDateFormat(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	stuID := uuid.New()
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/students/%s/attendance?from=nope", stuID), nil)
	apihandler.SetURLParamForTest(req, "id", stuID.String())
	req = authedReq(req, "center_admin")
	rec := httptest.NewRecorder()
	h.ListByStudent(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestAttendance_ListByOrg_Success_OptionalHalaqahFilter(t *testing.T) {
	svc := &stubAttendanceService{}
	h := apihandler.NewAttendanceHandler(svc)

	// Without halaqah filter.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/attendance?date_from=2026-05-01&date_to=2026-05-10", nil)
	req = authedReq(req, "center_admin")
	rec := httptest.NewRecorder()
	h.ListByOrg(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastListOrgH != nil {
		t.Fatalf("halaqah filter=%v want nil", svc.lastListOrgH)
	}

	// With halaqah filter.
	halaqahID := uuid.New()
	req2 := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/attendance?halaqah_id=%s&date_from=2026-05-01&date_to=2026-05-10", halaqahID), nil)
	req2 = authedReq(req2, "center_admin")
	rec2 := httptest.NewRecorder()
	h.ListByOrg(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	if svc.lastListOrgH == nil || *svc.lastListOrgH != halaqahID {
		t.Fatalf("halaqah filter=%v want %s", svc.lastListOrgH, halaqahID)
	}
}

func TestAttendance_ListByOrg_BadHalaqahFilter(t *testing.T) {
	h := apihandler.NewAttendanceHandler(&stubAttendanceService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/attendance?halaqah_id=nope", nil)
	req = authedReq(req, "center_admin")
	rec := httptest.NewRecorder()
	h.ListByOrg(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}
