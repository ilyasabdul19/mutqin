// api/internal/repo/platform_stats_test.go
package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestPlatformStatsRepo_Compute_AggregatesAcrossTenants(t *testing.T) {
	t.Cleanup(func() {
		truncateAll(t)
		_, _ = testAdmin.ExecContext(context.Background(), "TRUNCATE recitations, attendance RESTART IDENTITY CASCADE")
	})
	bg := context.Background()

	// Two active orgs and one suspended.
	orgA := &model.Organization{Name: "PA", Slug: "ps-a", Country: "SO", Tier: "free", Status: "active"}
	orgB := &model.Organization{Name: "PB", Slug: "ps-b", Country: "SO", Tier: "free", Status: "active"}
	orgC := &model.Organization{Name: "PC", Slug: "ps-c", Country: "SO", Tier: "free", Status: "suspended"}
	for _, o := range []*model.Organization{orgA, orgB, orgC} {
		if err := repo.NewOrganizationRepo(testAdmin).Create(bg, o); err != nil {
			t.Fatalf("create %s: %v", o.Slug, err)
		}
	}

	// Halaqat + students across A and B.
	hA := &model.Halaqah{OrganizationID: orgA.ID, Name: "HA", MaxCapacity: 30, Status: "active"}
	hB := &model.Halaqah{OrganizationID: orgB.ID, Name: "HB", MaxCapacity: 30, Status: "active"}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, hA); err != nil {
		t.Fatalf("hA: %v", err)
	}
	if err := repo.NewHalaqahRepo(testAdmin).Create(bg, hB); err != nil {
		t.Fatalf("hB: %v", err)
	}
	sr := repo.NewStudentRepo(testAdmin)
	studentsA := make([]uuid.UUID, 0, 2)
	for _, n := range []string{"a1", "a2"} {
		s := &model.Student{OrganizationID: orgA.ID, HalaqahID: &hA.ID, Name: n, Status: "active"}
		if err := sr.Create(bg, s); err != nil {
			t.Fatalf("student %s: %v", n, err)
		}
		studentsA = append(studentsA, s.ID)
	}
	sB := &model.Student{OrganizationID: orgB.ID, HalaqahID: &hB.ID, Name: "b1", Status: "active"}
	if err := sr.Create(bg, sB); err != nil {
		t.Fatalf("student b1: %v", err)
	}
	// One inactive student — must not count.
	sIn := &model.Student{OrganizationID: orgB.ID, HalaqahID: &hB.ID, Name: "b-inactive", Status: "inactive"}
	if err := sr.Create(bg, sIn); err != nil {
		t.Fatalf("inactive student: %v", err)
	}

	// Recitations — 3 recent (across A+B), 1 old (must not count).
	teacherA := seedTeacherForOrg(t, orgA.ID, "PS-A")
	teacherB := seedTeacherForOrg(t, orgB.ID, "PS-B")
	now := time.Now().UTC()
	seedRecitation(t, orgA.ID, hA.ID, studentsA[0], teacherA, now.AddDate(0, 0, -1))
	seedRecitation(t, orgA.ID, hA.ID, studentsA[1], teacherA, now.AddDate(0, 0, -2))
	seedRecitation(t, orgB.ID, hB.ID, sB.ID, teacherB, now.AddDate(0, 0, -5))
	seedRecitation(t, orgA.ID, hA.ID, studentsA[0], teacherA, now.AddDate(0, 0, -60))

	r := repo.NewPlatformStatsRepo(testAdmin)
	got, err := r.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got.Centers != 3 {
		t.Fatalf("Centers=%d want 3", got.Centers)
	}
	if got.ActiveCenters != 2 {
		t.Fatalf("ActiveCenters=%d want 2", got.ActiveCenters)
	}
	if got.SuspendedCenters != 1 {
		t.Fatalf("SuspendedCenters=%d want 1", got.SuspendedCenters)
	}
	if got.TotalStudents != 3 {
		t.Fatalf("TotalStudents=%d want 3", got.TotalStudents)
	}
	if got.TotalRecitations30d != 3 {
		t.Fatalf("TotalRecitations30d=%d want 3", got.TotalRecitations30d)
	}
}
