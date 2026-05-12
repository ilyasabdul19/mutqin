// api/internal/repo/attendance_test.go
package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

// attendanceFixture sets up an org + halaqah + 3 students and returns IDs.
type attendanceFixture struct {
	OrgID     uuid.UUID
	HalaqahID uuid.UUID
	Students  []uuid.UUID
}

func setupAttendanceFixture(t *testing.T, slug string) attendanceFixture {
	t.Helper()
	ctx := context.Background()
	org := &model.Organization{Name: "A Org " + slug, Slug: slug, Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	h := &model.Halaqah{OrganizationID: org.ID, Name: "Hal " + slug, MaxCapacity: 30}
	if err := repo.NewHalaqahRepo(testAdmin).Create(ctx, h); err != nil {
		t.Fatalf("create halaqah: %v", err)
	}
	sr := repo.NewStudentRepo(testAdmin)
	students := make([]uuid.UUID, 0, 3)
	for _, n := range []string{"Ali", "Omar", "Bilal"} {
		s := &model.Student{OrganizationID: org.ID, HalaqahID: &h.ID, Name: n}
		if err := sr.Create(ctx, s); err != nil {
			t.Fatalf("create student %s: %v", n, err)
		}
		students = append(students, s.ID)
	}
	return attendanceFixture{OrgID: org.ID, HalaqahID: h.ID, Students: students}
}

func TestAttendanceRepo_UpsertBatch_InsertsRows(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	fx := setupAttendanceFixture(t, "a-insert")

	r := repo.NewAttendanceRepo(testAdmin)
	date := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	rows := []model.Attendance{
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: date, Status: "present"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[1], Date: date, Status: "present"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[2], Date: date, Status: "absent"},
	}
	if err := r.UpsertBatch(ctx, rows); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	got, err := r.ListByHalaqahDate(tenant.With(ctx, fx.OrgID), fx.HalaqahID, date)
	if err != nil {
		t.Fatalf("ListByHalaqahDate: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestAttendanceRepo_UpsertBatch_UpdatesOnConflict(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	fx := setupAttendanceFixture(t, "a-update")

	r := repo.NewAttendanceRepo(testAdmin)
	date := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)

	// First pass: all present.
	first := []model.Attendance{
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: date, Status: "present"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[1], Date: date, Status: "present"},
	}
	if err := r.UpsertBatch(ctx, first); err != nil {
		t.Fatalf("UpsertBatch first: %v", err)
	}

	// Second pass: same students/date but one flips to absent.
	second := []model.Attendance{
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: date, Status: "absent"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[1], Date: date, Status: "present"},
	}
	if err := r.UpsertBatch(ctx, second); err != nil {
		t.Fatalf("UpsertBatch second: %v", err)
	}

	got, err := r.ListByHalaqahDate(tenant.With(ctx, fx.OrgID), fx.HalaqahID, date)
	if err != nil {
		t.Fatalf("ListByHalaqahDate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (UNIQUE collapses)", len(got))
	}
	// Find the one for Students[0] and confirm status flipped.
	var flipped *model.Attendance
	for i := range got {
		if got[i].StudentID == fx.Students[0] {
			flipped = &got[i]
		}
	}
	if flipped == nil {
		t.Fatal("Students[0] row missing")
	}
	if flipped.Status != "absent" {
		t.Fatalf("Students[0] status=%s want absent", flipped.Status)
	}
}

func TestAttendanceRepo_ListByHalaqahDate_FiltersByDateAndHalaqah(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	fx := setupAttendanceFixture(t, "a-listhd")

	r := repo.NewAttendanceRepo(testAdmin)
	dateA := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	dateB := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	rows := []model.Attendance{
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: dateA, Status: "present"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[1], Date: dateA, Status: "present"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: dateB, Status: "absent"},
	}
	if err := r.UpsertBatch(ctx, rows); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	gotA, err := r.ListByHalaqahDate(tenant.With(ctx, fx.OrgID), fx.HalaqahID, dateA)
	if err != nil {
		t.Fatalf("ListByHalaqahDate A: %v", err)
	}
	if len(gotA) != 2 {
		t.Fatalf("dateA: len=%d want 2", len(gotA))
	}
	gotB, err := r.ListByHalaqahDate(tenant.With(ctx, fx.OrgID), fx.HalaqahID, dateB)
	if err != nil {
		t.Fatalf("ListByHalaqahDate B: %v", err)
	}
	if len(gotB) != 1 {
		t.Fatalf("dateB: len=%d want 1", len(gotB))
	}
}

func TestAttendanceRepo_ListByStudent_DateRangeNarrows(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	fx := setupAttendanceFixture(t, "a-liststu")

	r := repo.NewAttendanceRepo(testAdmin)
	d1 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	d5 := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	d10 := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	rows := []model.Attendance{
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: d1, Status: "present"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: d5, Status: "absent"},
		{OrganizationID: fx.OrgID, HalaqahID: fx.HalaqahID, StudentID: fx.Students[0], Date: d10, Status: "present"},
	}
	if err := r.UpsertBatch(ctx, rows); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	// Narrow window — should capture only d5.
	got, err := r.ListByStudent(tenant.With(ctx, fx.OrgID), fx.Students[0],
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		100, 0)
	if err != nil {
		t.Fatalf("ListByStudent: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("narrow window: len=%d want 1", len(got))
	}
	if !got[0].Date.Equal(d5) {
		t.Fatalf("date=%v want %v", got[0].Date, d5)
	}

	// Wide window — all 3.
	all, err := r.ListByStudent(tenant.With(ctx, fx.OrgID), fx.Students[0],
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		100, 0)
	if err != nil {
		t.Fatalf("ListByStudent wide: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("wide window: len=%d want 3", len(all))
	}
}
