// api/internal/handler/api/dashboard_test.go
package api_test

import (
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
	"github.com/ilyas/mutqin-api/internal/repo"
)

type stubDashboardService struct {
	statsReturn *repo.CenterStats
	statsErr    error

	trendsHalaqah *uuid.UUID
	trendsFrom    time.Time
	trendsTo      time.Time
	trendsReturn  []repo.AttendanceTrendRow
	trendsErr     error

	actFrom   time.Time
	actTo     time.Time
	actReturn []repo.RecitationActivityRow
	actErr    error
}

func (s *stubDashboardService) CenterStats(_ context.Context) (*repo.CenterStats, error) {
	return s.statsReturn, s.statsErr
}
func (s *stubDashboardService) AttendanceTrends(_ context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error) {
	s.trendsHalaqah = halaqahID
	s.trendsFrom = from
	s.trendsTo = to
	return s.trendsReturn, s.trendsErr
}
func (s *stubDashboardService) RecitationActivity(_ context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error) {
	s.actFrom = from
	s.actTo = to
	return s.actReturn, s.actErr
}

func dashAuthedReq(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	return req
}

func TestDashboard_Stats_RequiresIdentity(t *testing.T) {
	h := apihandler.NewDashboardHandler(&stubDashboardService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestDashboard_Stats_HappyPath(t *testing.T) {
	want := &repo.CenterStats{Students: 3, Halaqat: 1, Teachers: 2, Recitations30d: 5, AttendanceRate30d: 0.8}
	svc := &stubDashboardService{statsReturn: want}
	h := apihandler.NewDashboardHandler(svc)
	rec := httptest.NewRecorder()
	h.Stats(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/stats"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data["students"].(float64) != 3 {
		t.Fatalf("students=%v", resp.Data["students"])
	}
	if resp.Data["attendance_rate_30d"].(float64) != 0.8 {
		t.Fatalf("rate=%v", resp.Data["attendance_rate_30d"])
	}
}

func TestDashboard_Stats_RepoErrorMapsTo500(t *testing.T) {
	svc := &stubDashboardService{statsErr: errors.New("db down")}
	h := apihandler.NewDashboardHandler(svc)
	rec := httptest.NewRecorder()
	h.Stats(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/stats"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestDashboard_AttendanceTrends_DefaultRange(t *testing.T) {
	svc := &stubDashboardService{trendsReturn: []repo.AttendanceTrendRow{{Total: 1}, {Total: 2}}}
	h := apihandler.NewDashboardHandler(svc)
	rec := httptest.NewRecorder()
	h.AttendanceTrends(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/attendance-trends"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.trendsHalaqah != nil {
		t.Fatalf("halaqah should be nil")
	}
	// Range defaults: today-30d .. today (UTC). Sanity-check the span.
	if svc.trendsTo.Sub(svc.trendsFrom) < 29*24*time.Hour || svc.trendsTo.Sub(svc.trendsFrom) > 31*24*time.Hour {
		t.Fatalf("default range span = %v", svc.trendsTo.Sub(svc.trendsFrom))
	}
}

func TestDashboard_AttendanceTrends_WithHalaqahFilter(t *testing.T) {
	svc := &stubDashboardService{trendsReturn: []repo.AttendanceTrendRow{{Total: 1}}}
	h := apihandler.NewDashboardHandler(svc)
	hid := uuid.New()
	req := dashAuthedReq(http.MethodGet,
		"/api/v1/dashboard/attendance-trends?from=2026-05-01&to=2026-05-10&halaqah_id="+hid.String())
	rec := httptest.NewRecorder()
	h.AttendanceTrends(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.trendsHalaqah == nil || *svc.trendsHalaqah != hid {
		t.Fatalf("halaqah mismatch: %v", svc.trendsHalaqah)
	}
	if svc.trendsFrom.Format("2006-01-02") != "2026-05-01" {
		t.Fatalf("from=%s", svc.trendsFrom)
	}
	if svc.trendsTo.Format("2006-01-02") != "2026-05-10" {
		t.Fatalf("to=%s", svc.trendsTo)
	}
}

func TestDashboard_AttendanceTrends_BadDate(t *testing.T) {
	h := apihandler.NewDashboardHandler(&stubDashboardService{})
	rec := httptest.NewRecorder()
	h.AttendanceTrends(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/attendance-trends?from=nope"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestDashboard_AttendanceTrends_BadHalaqahID(t *testing.T) {
	h := apihandler.NewDashboardHandler(&stubDashboardService{})
	rec := httptest.NewRecorder()
	h.AttendanceTrends(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/attendance-trends?halaqah_id=bad"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestDashboard_RecitationActivity_HappyPath(t *testing.T) {
	want := []repo.RecitationActivityRow{
		{HalaqahID: uuid.New(), HalaqahName: "A", Count: 5},
		{HalaqahID: uuid.New(), HalaqahName: "B", Count: 2},
	}
	svc := &stubDashboardService{actReturn: want}
	h := apihandler.NewDashboardHandler(svc)
	rec := httptest.NewRecorder()
	h.RecitationActivity(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/recitation-activity"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len=%d", len(resp.Data))
	}
	if resp.Meta["total"].(float64) != 2 {
		t.Fatalf("meta total=%v", resp.Meta["total"])
	}
}

func TestDashboard_RecitationActivity_BadDate(t *testing.T) {
	h := apihandler.NewDashboardHandler(&stubDashboardService{})
	rec := httptest.NewRecorder()
	h.RecitationActivity(rec, dashAuthedReq(http.MethodGet, "/api/v1/dashboard/recitation-activity?to=zzzz"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}
