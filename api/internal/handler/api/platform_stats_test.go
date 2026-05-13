// api/internal/handler/api/platform_stats_test.go
package api_test

import (
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
)

type stubPlatformStatsService struct {
	statsRet *repo.PlatformStats
	statsErr error

	auditActor  uuid.UUID
	auditLimit  int
	auditOffset int
	auditRet    []model.AuditLog
	auditErr    error
}

func (s *stubPlatformStatsService) Stats(_ context.Context) (*repo.PlatformStats, error) {
	return s.statsRet, s.statsErr
}
func (s *stubPlatformStatsService) AuditLog(_ context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error) {
	s.auditActor = actorID
	s.auditLimit = limit
	s.auditOffset = offset
	return s.auditRet, s.auditErr
}

func adminReq(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
}

func TestPlatformStats_Stats_RequiresIdentity(t *testing.T) {
	h := apihandler.NewPlatformStatsHandler(&stubPlatformStatsService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/stats", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestPlatformStats_Stats_HappyPath(t *testing.T) {
	want := &repo.PlatformStats{Centers: 5, ActiveCenters: 4, SuspendedCenters: 1, TotalStudents: 50, TotalRecitations30d: 100}
	svc := &stubPlatformStatsService{statsRet: want}
	h := apihandler.NewPlatformStatsHandler(svc)

	rec := httptest.NewRecorder()
	h.Stats(rec, adminReq(http.MethodGet, "/api/v1/platform/stats"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["centers"].(float64) != 5 {
		t.Fatalf("centers=%v", resp.Data["centers"])
	}
	if resp.Data["active_centers"].(float64) != 4 {
		t.Fatalf("active_centers=%v", resp.Data["active_centers"])
	}
}

func TestPlatformStats_Stats_ErrorMapsTo500(t *testing.T) {
	svc := &stubPlatformStatsService{statsErr: errors.New("db error")}
	h := apihandler.NewPlatformStatsHandler(svc)
	rec := httptest.NewRecorder()
	h.Stats(rec, adminReq(http.MethodGet, "/api/v1/platform/stats"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}

func TestPlatformStats_AuditLog_HappyPath(t *testing.T) {
	actor := uuid.New()
	want := []model.AuditLog{{ID: uuid.New(), Action: "create_organization"}}
	svc := &stubPlatformStatsService{auditRet: want}
	h := apihandler.NewPlatformStatsHandler(svc)

	rec := httptest.NewRecorder()
	h.AuditLog(rec, adminReq(http.MethodGet, "/api/v1/platform/audit-log?actor_id="+actor.String()+"&limit=10&offset=5"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.auditActor != actor {
		t.Fatalf("actor=%s want %s", svc.auditActor, actor)
	}
	if svc.auditLimit != 10 || svc.auditOffset != 5 {
		t.Fatalf("limit=%d offset=%d", svc.auditLimit, svc.auditOffset)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("len=%d", len(resp.Data))
	}
}

func TestPlatformStats_AuditLog_MissingActor_ReturnsEmptyList(t *testing.T) {
	svc := &stubPlatformStatsService{}
	h := apihandler.NewPlatformStatsHandler(svc)
	rec := httptest.NewRecorder()
	h.AuditLog(rec, adminReq(http.MethodGet, "/api/v1/platform/audit-log"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	// Service must NOT have been called.
	if svc.auditActor != (uuid.UUID{}) {
		t.Fatalf("service called: %v", svc.auditActor)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("len=%d want 0", len(resp.Data))
	}
}

func TestPlatformStats_AuditLog_BadActorID(t *testing.T) {
	h := apihandler.NewPlatformStatsHandler(&stubPlatformStatsService{})
	rec := httptest.NewRecorder()
	h.AuditLog(rec, adminReq(http.MethodGet, "/api/v1/platform/audit-log?actor_id=nope"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestPlatformStats_AuditLog_RequiresIdentity(t *testing.T) {
	h := apihandler.NewPlatformStatsHandler(&stubPlatformStatsService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/audit-log", nil)
	rec := httptest.NewRecorder()
	h.AuditLog(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}
