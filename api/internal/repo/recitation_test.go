// api/internal/repo/recitation_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

// recitationFixture seeds: org + halaqah + student + teacher user, returning
// the IDs needed to build Recitation rows. All setup uses testAdmin (superuser
// bypass — no RLS interference for fixtures).
type recitationFixture struct {
	orgID     uuid.UUID
	halaqahID uuid.UUID
	studentID uuid.UUID
	teacherID uuid.UUID
}

func seedRecitationFixture(t *testing.T, slugSuffix string) recitationFixture {
	t.Helper()
	ctx := context.Background()

	org := &model.Organization{Name: "R Org " + slugSuffix, Slug: "r-org-" + slugSuffix, Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	h := &model.Halaqah{OrganizationID: org.ID, Name: "Hal " + slugSuffix, MaxCapacity: 30}
	if err := repo.NewHalaqahRepo(testAdmin).Create(ctx, h); err != nil {
		t.Fatalf("create halaqah: %v", err)
	}
	s := &model.Student{OrganizationID: org.ID, HalaqahID: &h.ID, Name: "Student " + slugSuffix}
	if err := repo.NewStudentRepo(testAdmin).Create(ctx, s); err != nil {
		t.Fatalf("create student: %v", err)
	}
	u := &model.User{
		Name:           "Teacher " + slugSuffix,
		Email:          ptrString("teacher-" + slugSuffix + "@x"),
		Role:           "teacher",
		OrganizationID: ptrUUID(org.ID),
		Status:         "active",
		Language:       "ar",
	}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, u); err != nil {
		t.Fatalf("create teacher: %v", err)
	}
	return recitationFixture{orgID: org.ID, halaqahID: h.ID, studentID: s.ID, teacherID: u.ID}
}

func TestRecitationRepo_CreateAndGetLatest(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	f := seedRecitationFixture(t, "create-latest")
	ctx := context.Background()

	r := repo.NewRecitationRepo(testAdmin)
	rec := &model.Recitation{
		OrganizationID: f.orgID,
		StudentID:      f.studentID,
		HalaqahID:      f.halaqahID,
		TeacherID:      f.teacherID,
		Type:           "new_hifz",
		SurahNumber:    2,
		AyahFrom:       1,
		AyahTo:         5,
		Grade:          "mumtaz",
	}
	if err := r.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Fatal("ID not populated")
	}

	got, err := r.GetLatestByStudent(tenant.With(ctx, f.orgID), f.studentID)
	if err != nil {
		t.Fatalf("GetLatestByStudent: %v", err)
	}
	if got.ID != rec.ID {
		t.Fatalf("got id=%s want %s", got.ID, rec.ID)
	}
	if got.SurahNumber != 2 || got.AyahFrom != 1 || got.AyahTo != 5 {
		t.Fatalf("payload mismatch: %+v", got)
	}
}

func TestRecitationRepo_ListByStudent_DescOrder(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	f := seedRecitationFixture(t, "desc-order")
	ctx := context.Background()

	r := repo.NewRecitationRepo(testAdmin)
	for i, surah := range []int{1, 2, 3} {
		rec := &model.Recitation{
			OrganizationID: f.orgID,
			StudentID:      f.studentID,
			HalaqahID:      f.halaqahID,
			TeacherID:      f.teacherID,
			Type:           "new_hifz",
			SurahNumber:    surah,
			AyahFrom:       1,
			AyahTo:         3,
			Grade:          "jayyid",
		}
		if err := r.Create(ctx, rec); err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
		// Small sleep so recorded_at default(now()) differs between rows.
		time.Sleep(time.Millisecond)
	}

	got, err := r.ListByStudent(tenant.With(ctx, f.orgID), f.studentID, 100, 0)
	if err != nil {
		t.Fatalf("ListByStudent: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	// Newest first: insertion order was surah 1, 2, 3 → expect 3, 2, 1.
	if got[0].SurahNumber != 3 || got[1].SurahNumber != 2 || got[2].SurahNumber != 1 {
		t.Fatalf("order: [%d, %d, %d] want [3, 2, 1]", got[0].SurahNumber, got[1].SurahNumber, got[2].SurahNumber)
	}
}

func TestRecitationRepo_BatchCreate(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	f := seedRecitationFixture(t, "batch")
	ctx := context.Background()

	r := repo.NewRecitationRepo(testAdmin)
	batch := []model.Recitation{
		{OrganizationID: f.orgID, StudentID: f.studentID, HalaqahID: f.halaqahID, TeacherID: f.teacherID, Type: "new_hifz", SurahNumber: 1, AyahFrom: 1, AyahTo: 7, Grade: "mumtaz"},
		{OrganizationID: f.orgID, StudentID: f.studentID, HalaqahID: f.halaqahID, TeacherID: f.teacherID, Type: "near_review", SurahNumber: 2, AyahFrom: 1, AyahTo: 5, Grade: "jayyid"},
		{OrganizationID: f.orgID, StudentID: f.studentID, HalaqahID: f.halaqahID, TeacherID: f.teacherID, Type: "far_review", SurahNumber: 3, AyahFrom: 1, AyahTo: 10, Grade: "maqbul"},
	}
	if err := r.BatchCreate(ctx, batch); err != nil {
		t.Fatalf("BatchCreate: %v", err)
	}

	got, err := r.ListByStudent(tenant.With(ctx, f.orgID), f.studentID, 100, 0)
	if err != nil {
		t.Fatalf("ListByStudent: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestRecitationRepo_GetLatest_NotFound(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	f := seedRecitationFixture(t, "not-found")
	ctx := context.Background()

	r := repo.NewRecitationRepo(testAdmin)
	_, err := r.GetLatestByStudent(tenant.With(ctx, f.orgID), f.studentID)
	if !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestRecitationRepo_ListByOrgSince_FiltersAndOrdersAsc(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	f := seedRecitationFixture(t, "since")
	ctx := context.Background()

	r := repo.NewRecitationRepo(testAdmin)

	older := time.Now().Add(-2 * time.Hour)
	mid := time.Now().Add(-1 * time.Hour)
	newer := time.Now().Add(-10 * time.Minute)
	for i, when := range []time.Time{older, mid, newer} {
		rec := &model.Recitation{
			OrganizationID: f.orgID,
			StudentID:      f.studentID,
			HalaqahID:      f.halaqahID,
			TeacherID:      f.teacherID,
			Type:           "new_hifz",
			SurahNumber:    i + 1,
			AyahFrom:       1,
			AyahTo:         3,
			Grade:          "jayyid",
			RecordedAt:     when,
		}
		if err := r.Create(ctx, rec); err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
	}

	since := time.Now().Add(-90 * time.Minute) // strictly between older and mid
	got, err := r.ListByOrgSince(tenant.With(ctx, f.orgID), since, 100)
	if err != nil {
		t.Fatalf("ListByOrgSince: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (mid + newer)", len(got))
	}
	// Ascending: mid first, then newer.
	if !got[0].RecordedAt.Before(got[1].RecordedAt) {
		t.Fatalf("not ascending: %v then %v", got[0].RecordedAt, got[1].RecordedAt)
	}
}

func TestRecitationRepo_ListByOrgSince_ClampsLimit(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	f := seedRecitationFixture(t, "since-clamp")
	ctx := context.Background()

	r := repo.NewRecitationRepo(testAdmin)
	// Empty table — just exercise limit clamp via no error.
	got, err := r.ListByOrgSince(tenant.With(ctx, f.orgID), time.Unix(0, 0), 0)
	if err != nil {
		t.Fatalf("ListByOrgSince(limit=0): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len=%d want 0", len(got))
	}
	got, err = r.ListByOrgSince(tenant.With(ctx, f.orgID), time.Unix(0, 0), 10000)
	if err != nil {
		t.Fatalf("ListByOrgSince(limit=10000): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len=%d want 0 (still empty)", len(got))
	}
}
