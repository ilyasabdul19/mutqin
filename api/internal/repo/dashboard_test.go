// api/internal/repo/dashboard_test.go
package repo_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

// withTenantTx begins a transaction on testDB, sets app.current_tenant, and
// installs the tx into ctx so repos picked up via db.TxFrom run under RLS.
// The returned cleanup rolls the tx back (tests don't need to commit).
func withTenantTx(t *testing.T, orgID uuid.UUID) (context.Context, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant = '"+orgID.String()+"'"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("set local: %v", err)
	}
	scopedCtx := db.WithTx(ctx, bun.IDB(tx))
	cleanup := func() { _ = tx.Rollback() }
	return scopedCtx, cleanup
}

func seedRecitation(t *testing.T, orgID, halaqahID, studentID uuid.UUID, recordedAt time.Time) {
	t.Helper()
	_, err := testAdmin.ExecContext(context.Background(),
		`INSERT INTO recitations (organization_id, halaqah_id, student_id, surah, ayah_from, ayah_to, recorded_at)
		 VALUES (?, ?, ?, 'al-fatihah', 1, 7, ?)`,
		orgID, halaqahID, studentID, recordedAt)
	if err != nil {
		t.Fatalf("seed recitation: %v", err)
	}
}

func TestDashboardRepo_CenterStats_CountsActiveAndRecent(t *testing.T) {
	t.Cleanup(func() {
		truncateAll(t)
		_, _ = testAdmin.ExecContext(context.Background(), "TRUNCATE recitations, attendance RESTART IDENTITY CASCADE")
	})
	bg := context.Background()

	orgA := &model.Organization{Name: "DashA", Slug: "dash-stats-a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(bg, orgA); err != nil {
		t.Fatalf("create orgA: %v", err)
	}
	orgB := &model.Organization{Name: "DashB", Slug: "dash-stats-b", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(bg, orgB); err != nil {
		t.Fatalf("create orgB: %v", err)
	}

	// orgA fixtures.
	hA := &model.Halaqah{OrganizationID: orgA.ID, Name: "HA", MaxCapacity: 30, Status: "active"}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, hA); err != nil {
		t.Fatalf("create hA: %v", err)
	}
	teacherA := &model.User{Name: "TA", Email: ptrString("dash-tA@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(bg, teacherA); err != nil {
		t.Fatalf("create teacherA: %v", err)
	}
	sr := repo.NewStudentRepo(testAdmin)
	studentIDs := make([]uuid.UUID, 0, 3)
	for _, n := range []string{"S1", "S2", "S3"} {
		s := &model.Student{OrganizationID: orgA.ID, HalaqahID: &hA.ID, Name: n, Status: "active"}
		if err := sr.Create(bg, s); err != nil {
			t.Fatalf("create student %s: %v", n, err)
		}
		studentIDs = append(studentIDs, s.ID)
	}

	// orgB fixtures — should NOT show up in orgA stats.
	hB := &model.Halaqah{OrganizationID: orgB.ID, Name: "HB", MaxCapacity: 30, Status: "active"}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, hB); err != nil {
		t.Fatalf("create hB: %v", err)
	}
	sB := &model.Student{OrganizationID: orgB.ID, HalaqahID: &hB.ID, Name: "B1", Status: "active"}
	if err := sr.Create(bg, sB); err != nil {
		t.Fatalf("create student B1: %v", err)
	}

	// Recitations for orgA — 2 in last 30 days, 1 older.
	now := time.Now().UTC()
	seedRecitation(t, orgA.ID, hA.ID, studentIDs[0], now.AddDate(0, 0, -3))
	seedRecitation(t, orgA.ID, hA.ID, studentIDs[1], now.AddDate(0, 0, -10))
	seedRecitation(t, orgA.ID, hA.ID, studentIDs[2], now.AddDate(0, 0, -60))
	// orgB recitation — must not bleed through.
	seedRecitation(t, orgB.ID, hB.ID, sB.ID, now.AddDate(0, 0, -1))

	// Attendance — 3 present, 1 absent on a recent date.
	today := now.Truncate(24 * time.Hour)
	rows := []model.Attendance{
		{OrganizationID: orgA.ID, HalaqahID: hA.ID, StudentID: studentIDs[0], Date: today, Status: "present"},
		{OrganizationID: orgA.ID, HalaqahID: hA.ID, StudentID: studentIDs[1], Date: today, Status: "present"},
		{OrganizationID: orgA.ID, HalaqahID: hA.ID, StudentID: studentIDs[2], Date: today, Status: "absent"},
	}
	if err := repo.NewAttendanceRepo(testAdmin).UpsertBatch(bg, rows); err != nil {
		t.Fatalf("seed attendance: %v", err)
	}

	scopedCtx, cleanup := withTenantTx(t, orgA.ID)
	defer cleanup()

	r := repo.NewDashboardRepo(testDB)
	got, err := r.CenterStats(scopedCtx)
	if err != nil {
		t.Fatalf("CenterStats: %v", err)
	}
	if got.Students != 3 {
		t.Fatalf("Students=%d want 3", got.Students)
	}
	if got.Halaqat != 1 {
		t.Fatalf("Halaqat=%d want 1", got.Halaqat)
	}
	if got.Teachers != 1 {
		t.Fatalf("Teachers=%d want 1", got.Teachers)
	}
	if got.Recitations30d != 2 {
		t.Fatalf("Recitations30d=%d want 2", got.Recitations30d)
	}
	// 2 present / 3 total = ~0.6666...
	if got.AttendanceRate30d < 0.6 || got.AttendanceRate30d > 0.7 {
		t.Fatalf("AttendanceRate30d=%f want ~0.6667", got.AttendanceRate30d)
	}
}

func TestDashboardRepo_AttendanceTrends_GroupsByDate(t *testing.T) {
	t.Cleanup(func() {
		truncateAll(t)
		_, _ = testAdmin.ExecContext(context.Background(), "TRUNCATE recitations, attendance RESTART IDENTITY CASCADE")
	})
	bg := context.Background()

	org := &model.Organization{Name: "DT", Slug: "dt-trends", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(bg, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	h := &model.Halaqah{OrganizationID: org.ID, Name: "HT", MaxCapacity: 30, Status: "active"}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, h); err != nil {
		t.Fatalf("create halaqah: %v", err)
	}
	sr := repo.NewStudentRepo(testAdmin)
	students := make([]uuid.UUID, 0, 3)
	for _, n := range []string{"a", "b", "c"} {
		s := &model.Student{OrganizationID: org.ID, HalaqahID: &h.ID, Name: n, Status: "active"}
		if err := sr.Create(bg, s); err != nil {
			t.Fatalf("create student: %v", err)
		}
		students = append(students, s.ID)
	}

	d1 := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	rows := []model.Attendance{
		// d1: 2 present, 1 absent
		{OrganizationID: org.ID, HalaqahID: h.ID, StudentID: students[0], Date: d1, Status: "present"},
		{OrganizationID: org.ID, HalaqahID: h.ID, StudentID: students[1], Date: d1, Status: "present"},
		{OrganizationID: org.ID, HalaqahID: h.ID, StudentID: students[2], Date: d1, Status: "absent"},
		// d2: 1 present, 2 absent
		{OrganizationID: org.ID, HalaqahID: h.ID, StudentID: students[0], Date: d2, Status: "absent"},
		{OrganizationID: org.ID, HalaqahID: h.ID, StudentID: students[1], Date: d2, Status: "present"},
		{OrganizationID: org.ID, HalaqahID: h.ID, StudentID: students[2], Date: d2, Status: "absent"},
	}
	if err := repo.NewAttendanceRepo(testAdmin).UpsertBatch(bg, rows); err != nil {
		t.Fatalf("seed attendance: %v", err)
	}

	scopedCtx, cleanup := withTenantTx(t, org.ID)
	defer cleanup()

	r := repo.NewDashboardRepo(testDB)
	got, err := r.AttendanceTrends(scopedCtx, nil, d1, d2)
	if err != nil {
		t.Fatalf("AttendanceTrends: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	// First row d1: present=2 absent=1 total=3
	if got[0].PresentCount != 2 || got[0].AbsentCount != 1 || got[0].Total != 3 {
		t.Fatalf("d1 row: %+v", got[0])
	}
	if got[1].PresentCount != 1 || got[1].AbsentCount != 2 || got[1].Total != 3 {
		t.Fatalf("d2 row: %+v", got[1])
	}

	// With halaqah filter — still both rows.
	gotF, err := r.AttendanceTrends(scopedCtx, &h.ID, d1, d2)
	if err != nil {
		t.Fatalf("AttendanceTrends filter: %v", err)
	}
	if len(gotF) != 2 {
		t.Fatalf("filter len=%d want 2", len(gotF))
	}
}

func TestDashboardRepo_RecitationActivity_OrdersByCountDesc(t *testing.T) {
	t.Cleanup(func() {
		truncateAll(t)
		_, _ = testAdmin.ExecContext(context.Background(), "TRUNCATE recitations, attendance RESTART IDENTITY CASCADE")
	})
	bg := context.Background()

	org := &model.Organization{Name: "RA", Slug: "ra-act", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(bg, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	hLow := &model.Halaqah{OrganizationID: org.ID, Name: "LowHal", MaxCapacity: 30, Status: "active"}
	hHigh := &model.Halaqah{OrganizationID: org.ID, Name: "HighHal", MaxCapacity: 30, Status: "active"}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, hLow); err != nil {
		t.Fatalf("create hLow: %v", err)
	}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, hHigh); err != nil {
		t.Fatalf("create hHigh: %v", err)
	}
	sr := repo.NewStudentRepo(testAdmin)
	s1 := &model.Student{OrganizationID: org.ID, HalaqahID: &hLow.ID, Name: "s1", Status: "active"}
	s2 := &model.Student{OrganizationID: org.ID, HalaqahID: &hHigh.ID, Name: "s2", Status: "active"}
	if err := sr.Create(bg, s1); err != nil {
		t.Fatalf("s1: %v", err)
	}
	if err := sr.Create(bg, s2); err != nil {
		t.Fatalf("s2: %v", err)
	}

	now := time.Now().UTC()
	// hHigh gets 3 recitations, hLow gets 1.
	for i := 0; i < 3; i++ {
		seedRecitation(t, org.ID, hHigh.ID, s2.ID, now.AddDate(0, 0, -i))
	}
	seedRecitation(t, org.ID, hLow.ID, s1.ID, now.AddDate(0, 0, -1))

	scopedCtx, cleanup := withTenantTx(t, org.ID)
	defer cleanup()

	r := repo.NewDashboardRepo(testDB)
	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, 1)
	got, err := r.RecitationActivity(scopedCtx, from, to)
	if err != nil {
		t.Fatalf("RecitationActivity: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	// Ordering: hHigh (3) before hLow (1).
	if got[0].HalaqahID != hHigh.ID {
		t.Fatalf("first row halaqah=%s want %s", got[0].HalaqahID, hHigh.ID)
	}
	if got[0].Count != 3 {
		t.Fatalf("first row count=%d want 3", got[0].Count)
	}
	if got[1].HalaqahID != hLow.ID {
		t.Fatalf("second row halaqah=%s want %s", got[1].HalaqahID, hLow.ID)
	}
	if got[1].Count != 1 {
		t.Fatalf("second row count=%d want 1", got[1].Count)
	}
}
