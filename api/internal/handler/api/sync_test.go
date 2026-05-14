// api/internal/handler/api/sync_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type stubSyncService struct {
	pushActor  uuid.UUID
	pushOrg    uuid.UUID
	pushInput  service.PushInput
	pushResult *service.PushResult
	pushErr    error

	pullSince  time.Time
	pullLimit  int
	pullResult *service.PullResult
	pullErr    error
}

func (s *stubSyncService) Push(_ context.Context, actor, orgID uuid.UUID, in service.PushInput) (*service.PushResult, error) {
	s.pushActor = actor
	s.pushOrg = orgID
	s.pushInput = in
	if s.pushErr != nil {
		return nil, s.pushErr
	}
	if s.pushResult != nil {
		return s.pushResult, nil
	}
	return &service.PushResult{
		RecitationsAccepted: len(in.Recitations),
		AttendanceAccepted:  len(in.Attendance),
		ServerNow:           time.Now().UTC(),
	}, nil
}

func (s *stubSyncService) Pull(_ context.Context, since time.Time, limit int) (*service.PullResult, error) {
	s.pullSince = since
	s.pullLimit = limit
	if s.pullErr != nil {
		return nil, s.pullErr
	}
	if s.pullResult != nil {
		return s.pullResult, nil
	}
	return &service.PullResult{ServerNow: time.Now().UTC()}, nil
}

func TestSync_Push_RequiresIdentity(t *testing.T) {
	h := apihandler.NewSyncHandler(&stubSyncService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/push", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestSync_Push_RequiresOrgID(t *testing.T) {
	h := apihandler.NewSyncHandler(&stubSyncService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/push", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSync_Push_BadJSONIs400(t *testing.T) {
	h := apihandler.NewSyncHandler(&stubSyncService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/push", bytes.NewReader([]byte("{not json")))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestSync_Push_HappyPath(t *testing.T) {
	svc := &stubSyncService{}
	h := apihandler.NewSyncHandler(svc)

	body := map[string]any{
		"recitations": []map[string]any{
			{
				"student_id":   uuid.New().String(),
				"halaqah_id":   uuid.New().String(),
				"type":         "new_hifz",
				"surah_number": 1,
				"ayah_from":    1,
				"ayah_to":      3,
				"grade":        "mumtaz",
			},
		},
		"attendance": []map[string]any{
			{
				"halaqah_id": uuid.New().String(),
				"student_id": uuid.New().String(),
				"date":       "2026-05-12T00:00:00Z",
				"status":     "present",
			},
		},
	}
	buf, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/push", bytes.NewReader(buf))
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Push(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.pushActor != actor || svc.pushOrg != orgID {
		t.Fatalf("actor/org mismatch: actor=%s org=%s", svc.pushActor, svc.pushOrg)
	}
	if len(svc.pushInput.Recitations) != 1 {
		t.Fatalf("recitations len=%d want 1", len(svc.pushInput.Recitations))
	}
	if len(svc.pushInput.Attendance) != 1 {
		t.Fatalf("attendance len=%d want 1", len(svc.pushInput.Attendance))
	}

	var resp struct {
		Data struct {
			RecitationsAccepted int `json:"recitations_accepted"`
			AttendanceAccepted  int `json:"attendance_accepted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if resp.Data.RecitationsAccepted != 1 || resp.Data.AttendanceAccepted != 1 {
		t.Fatalf("counts: %+v", resp.Data)
	}
}

func TestSync_Push_InvalidInputIs400(t *testing.T) {
	svc := &stubSyncService{pushErr: service.ErrInvalidSyncInput}
	h := apihandler.NewSyncHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/push", bytes.NewReader([]byte(`{}`)))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSync_Push_InfraErrorIs500(t *testing.T) {
	svc := &stubSyncService{pushErr: errors.New("db boom")}
	h := apihandler.NewSyncHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync/push", bytes.NewReader([]byte(`{}`)))
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Push(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}

func TestSync_Pull_RequiresIdentity(t *testing.T) {
	h := apihandler.NewSyncHandler(&stubSyncService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	rec := httptest.NewRecorder()
	h.Pull(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestSync_Pull_RequiresOrgID(t *testing.T) {
	h := apihandler.NewSyncHandler(&stubSyncService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.Pull(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestSync_Pull_ParsesSinceAndLimit(t *testing.T) {
	svc := &stubSyncService{pullResult: &service.PullResult{
		Recitations: []model.Recitation{{ID: uuid.New()}},
		Attendance:  []model.Attendance{{ID: uuid.New()}, {ID: uuid.New()}},
		ServerNow:   time.Now().UTC(),
	}}
	h := apihandler.NewSyncHandler(svc)

	since := "2026-05-12T03:04:05Z"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull?since="+since+"&limit=42", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Pull(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	want, _ := time.Parse(time.RFC3339, since)
	if !svc.pullSince.Equal(want) {
		t.Fatalf("since=%v want %v", svc.pullSince, want)
	}
	if svc.pullLimit != 42 {
		t.Fatalf("limit=%d want 42", svc.pullLimit)
	}
}

func TestSync_Pull_MissingSinceDefaultsToEpoch(t *testing.T) {
	svc := &stubSyncService{}
	h := apihandler.NewSyncHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Pull(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !svc.pullSince.Equal(time.Unix(0, 0)) {
		t.Fatalf("since=%v want epoch", svc.pullSince)
	}
}

func TestSync_Pull_BadSinceIs400(t *testing.T) {
	h := apihandler.NewSyncHandler(&stubSyncService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull?since=not-a-time", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Pull(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestSync_Pull_InfraErrorIs500(t *testing.T) {
	svc := &stubSyncService{pullErr: errors.New("boom")}
	h := apihandler.NewSyncHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "teacher"}))
	rec := httptest.NewRecorder()
	h.Pull(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}
