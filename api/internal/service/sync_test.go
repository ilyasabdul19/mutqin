// api/internal/service/sync_test.go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubSyncRecitationRepo struct {
	batchCalls [][]model.Recitation
	sinceCalls []time.Time
	sinceLimit int
	sinceOut   []model.Recitation
}

func (s *stubSyncRecitationRepo) BatchCreate(_ context.Context, rs []model.Recitation) error {
	cp := make([]model.Recitation, len(rs))
	copy(cp, rs)
	s.batchCalls = append(s.batchCalls, cp)
	return nil
}

func (s *stubSyncRecitationRepo) ListByOrgSince(_ context.Context, since time.Time, limit int) ([]model.Recitation, error) {
	s.sinceCalls = append(s.sinceCalls, since)
	s.sinceLimit = limit
	return s.sinceOut, nil
}

type stubSyncAttendanceRepo struct {
	upsertCalls [][]model.Attendance
	sinceCalls  []time.Time
	sinceLimit  int
	sinceOut    []model.Attendance
}

func (s *stubSyncAttendanceRepo) UpsertBatch(_ context.Context, rs []model.Attendance) error {
	cp := make([]model.Attendance, len(rs))
	copy(cp, rs)
	s.upsertCalls = append(s.upsertCalls, cp)
	return nil
}

func (s *stubSyncAttendanceRepo) ListByOrgSince(_ context.Context, since time.Time, limit int) ([]model.Attendance, error) {
	s.sinceCalls = append(s.sinceCalls, since)
	s.sinceLimit = limit
	return s.sinceOut, nil
}

type stubSyncConflictRepo struct{ created []model.SyncConflict }

func (s *stubSyncConflictRepo) Create(_ context.Context, c *model.SyncConflict) error {
	c.ID = uuid.New()
	s.created = append(s.created, *c)
	return nil
}

func TestSyncService_Push_StampsOrgAndTeacherAndInvokesRepos(t *testing.T) {
	recs := &stubSyncRecitationRepo{}
	att := &stubSyncAttendanceRepo{}
	conf := &stubSyncConflictRepo{}
	audit := &stubAuditRepo{}
	svc := service.NewSyncService(recs, att, conf, audit)

	actor := uuid.New()
	orgID := uuid.New()
	otherOrg := uuid.New()
	otherTeacher := uuid.New()

	in := service.PushInput{
		Recitations: []model.Recitation{
			// Client claims a different org + teacher — must be overwritten.
			{OrganizationID: otherOrg, TeacherID: otherTeacher, StudentID: uuid.New(), HalaqahID: uuid.New(), Type: "new_hifz", SurahNumber: 1, AyahFrom: 1, AyahTo: 3, Grade: "mumtaz"},
			{OrganizationID: otherOrg, TeacherID: otherTeacher, StudentID: uuid.New(), HalaqahID: uuid.New(), Type: "near_review", SurahNumber: 2, AyahFrom: 1, AyahTo: 5, Grade: "jayyid"},
		},
		Attendance: []model.Attendance{
			{OrganizationID: otherOrg, HalaqahID: uuid.New(), StudentID: uuid.New(), Date: time.Now(), Status: "present"},
		},
	}

	res, err := svc.Push(context.Background(), actor, orgID, in)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if res.RecitationsAccepted != 2 {
		t.Fatalf("recitations_accepted=%d want 2", res.RecitationsAccepted)
	}
	if res.AttendanceAccepted != 1 {
		t.Fatalf("attendance_accepted=%d want 1", res.AttendanceAccepted)
	}
	if res.ServerNow.IsZero() {
		t.Fatal("ServerNow not populated")
	}

	if len(recs.batchCalls) != 1 {
		t.Fatalf("BatchCreate calls=%d want 1", len(recs.batchCalls))
	}
	for i, r := range recs.batchCalls[0] {
		if r.OrganizationID != orgID {
			t.Fatalf("rec[%d] org=%s want %s (overwrite)", i, r.OrganizationID, orgID)
		}
		if r.TeacherID != actor {
			t.Fatalf("rec[%d] teacher=%s want %s (overwrite)", i, r.TeacherID, actor)
		}
		if r.SyncedAt == nil {
			t.Fatalf("rec[%d] synced_at not stamped", i)
		}
	}

	if len(att.upsertCalls) != 1 {
		t.Fatalf("UpsertBatch calls=%d want 1", len(att.upsertCalls))
	}
	for i, a := range att.upsertCalls[0] {
		if a.OrganizationID != orgID {
			t.Fatalf("att[%d] org=%s want %s (overwrite)", i, a.OrganizationID, orgID)
		}
		if a.SyncedAt == nil {
			t.Fatalf("att[%d] synced_at not stamped", i)
		}
	}
}

func TestSyncService_Push_AuditOnce(t *testing.T) {
	recs := &stubSyncRecitationRepo{}
	att := &stubSyncAttendanceRepo{}
	conf := &stubSyncConflictRepo{}
	audit := &stubAuditRepo{}
	svc := service.NewSyncService(recs, att, conf, audit)

	actor := uuid.New()
	orgID := uuid.New()
	_, err := svc.Push(context.Background(), actor, orgID, service.PushInput{
		Recitations: []model.Recitation{
			{StudentID: uuid.New(), HalaqahID: uuid.New(), Type: "new_hifz", SurahNumber: 1, AyahFrom: 1, AyahTo: 3, Grade: "mumtaz"},
		},
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "sync_push" {
		t.Fatalf("audit action=%s want sync_push", audit.logs[0].Action)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("audit actor mismatch")
	}
}

func TestSyncService_Push_SkipsRepoCallsForEmptySlices(t *testing.T) {
	recs := &stubSyncRecitationRepo{}
	att := &stubSyncAttendanceRepo{}
	conf := &stubSyncConflictRepo{}
	audit := &stubAuditRepo{}
	svc := service.NewSyncService(recs, att, conf, audit)

	if _, err := svc.Push(context.Background(), uuid.New(), uuid.New(), service.PushInput{}); err != nil {
		t.Fatalf("Push empty: %v", err)
	}
	if len(recs.batchCalls) != 0 {
		t.Fatalf("BatchCreate calls=%d want 0 for empty input", len(recs.batchCalls))
	}
	if len(att.upsertCalls) != 0 {
		t.Fatalf("UpsertBatch calls=%d want 0 for empty input", len(att.upsertCalls))
	}
}

func TestSyncService_Pull_CombinesListsAndStampsServerNow(t *testing.T) {
	recOut := []model.Recitation{{ID: uuid.New(), Grade: "mumtaz"}}
	attOut := []model.Attendance{{ID: uuid.New(), Status: "present"}, {ID: uuid.New(), Status: "absent"}}
	recs := &stubSyncRecitationRepo{sinceOut: recOut}
	att := &stubSyncAttendanceRepo{sinceOut: attOut}
	conf := &stubSyncConflictRepo{}
	audit := &stubAuditRepo{}
	svc := service.NewSyncService(recs, att, conf, audit)

	since := time.Now().Add(-time.Hour)
	res, err := svc.Pull(context.Background(), since, 50)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(res.Recitations) != 1 {
		t.Fatalf("recitations len=%d", len(res.Recitations))
	}
	if len(res.Attendance) != 2 {
		t.Fatalf("attendance len=%d", len(res.Attendance))
	}
	if res.ServerNow.IsZero() {
		t.Fatal("ServerNow not populated")
	}
	if len(recs.sinceCalls) != 1 || !recs.sinceCalls[0].Equal(since) {
		t.Fatalf("recs since not forwarded: %v", recs.sinceCalls)
	}
	if recs.sinceLimit != 50 || att.sinceLimit != 50 {
		t.Fatalf("limit not forwarded: rec=%d att=%d", recs.sinceLimit, att.sinceLimit)
	}
}
