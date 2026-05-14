// api/internal/service/dashboard_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubDashboardRepo struct {
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

func (s *stubDashboardRepo) CenterStats(_ context.Context) (*repo.CenterStats, error) {
	return s.statsReturn, s.statsErr
}

func (s *stubDashboardRepo) AttendanceTrends(_ context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error) {
	s.trendsHalaqah = halaqahID
	s.trendsFrom = from
	s.trendsTo = to
	return s.trendsReturn, s.trendsErr
}

func (s *stubDashboardRepo) RecitationActivity(_ context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error) {
	s.actFrom = from
	s.actTo = to
	return s.actReturn, s.actErr
}

type stubPlatformStatsRepo struct {
	ret *repo.PlatformStats
	err error
}

func (s *stubPlatformStatsRepo) Compute(_ context.Context) (*repo.PlatformStats, error) {
	return s.ret, s.err
}

type stubAuditLister struct {
	actor  uuid.UUID
	limit  int
	offset int
	ret    []model.AuditLog
	err    error
}

func (s *stubAuditLister) ListByActor(_ context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error) {
	s.actor = actorID
	s.limit = limit
	s.offset = offset
	return s.ret, s.err
}

func TestDashboardService_CenterStats_PassThrough(t *testing.T) {
	want := &repo.CenterStats{Students: 5, Halaqat: 2, Teachers: 1, Recitations30d: 7, AttendanceRate30d: 0.5}
	st := &stubDashboardRepo{statsReturn: want}
	svc := service.NewDashboardService(st)
	got, err := svc.CenterStats(context.Background())
	if err != nil {
		t.Fatalf("CenterStats: %v", err)
	}
	if got != want {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestDashboardService_AttendanceTrends_DelegatesArgs(t *testing.T) {
	st := &stubDashboardRepo{trendsReturn: []repo.AttendanceTrendRow{{Total: 3}}}
	svc := service.NewDashboardService(st)
	hid := uuid.New()
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	got, err := svc.AttendanceTrends(context.Background(), &hid, from, to)
	if err != nil {
		t.Fatalf("AttendanceTrends: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if st.trendsHalaqah == nil || *st.trendsHalaqah != hid {
		t.Fatalf("halaqah mismatch: %v", st.trendsHalaqah)
	}
	if !st.trendsFrom.Equal(from) || !st.trendsTo.Equal(to) {
		t.Fatalf("range mismatch: %v..%v", st.trendsFrom, st.trendsTo)
	}
}

func TestDashboardService_RecitationActivity_DelegatesRange(t *testing.T) {
	st := &stubDashboardRepo{actReturn: []repo.RecitationActivityRow{{Count: 2}}}
	svc := service.NewDashboardService(st)
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	got, err := svc.RecitationActivity(context.Background(), from, to)
	if err != nil {
		t.Fatalf("RecitationActivity: %v", err)
	}
	if len(got) != 1 || got[0].Count != 2 {
		t.Fatalf("got=%+v", got)
	}
	if !st.actFrom.Equal(from) || !st.actTo.Equal(to) {
		t.Fatalf("range mismatch")
	}
}

func TestDashboardService_PropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	st := &stubDashboardRepo{statsErr: sentinel}
	svc := service.NewDashboardService(st)
	_, err := svc.CenterStats(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want %v", err, sentinel)
	}
}

func TestPlatformService_Stats_PassThrough(t *testing.T) {
	want := &repo.PlatformStats{Centers: 4, ActiveCenters: 3}
	pr := &stubPlatformStatsRepo{ret: want}
	ar := &stubAuditLister{}
	svc := service.NewPlatformService(pr, ar)
	got, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got != want {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestPlatformService_AuditLog_ClampsLimit(t *testing.T) {
	pr := &stubPlatformStatsRepo{}
	ar := &stubAuditLister{ret: []model.AuditLog{{Action: "x"}}}
	svc := service.NewPlatformService(pr, ar)

	actor := uuid.New()
	cases := []struct {
		in, want int
	}{
		{0, 50},     // zero → default
		{-5, 50},    // negative → default
		{250, 50},   // over cap → default
		{200, 200},  // at cap → kept
		{42, 42},    // sane → kept
	}
	for _, c := range cases {
		if _, err := svc.AuditLog(context.Background(), actor, c.in, 0); err != nil {
			t.Fatalf("AuditLog in=%d: %v", c.in, err)
		}
		if ar.limit != c.want {
			t.Fatalf("limit in=%d got=%d want=%d", c.in, ar.limit, c.want)
		}
		if ar.actor != actor {
			t.Fatalf("actor mismatch")
		}
	}
}

func TestPlatformService_AuditLog_DelegatesOffset(t *testing.T) {
	pr := &stubPlatformStatsRepo{}
	ar := &stubAuditLister{}
	svc := service.NewPlatformService(pr, ar)
	if _, err := svc.AuditLog(context.Background(), uuid.New(), 25, 100); err != nil {
		t.Fatalf("AuditLog: %v", err)
	}
	if ar.offset != 100 {
		t.Fatalf("offset=%d want 100", ar.offset)
	}
}
