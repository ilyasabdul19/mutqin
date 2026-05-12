// api/internal/service/recitation_test.go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubRecitationRepo struct {
	stored          []model.Recitation
	byStudent       map[uuid.UUID][]model.Recitation
	lastListLimit   int
	lastListOffset  int
	lastListStudent uuid.UUID
	getLatestErr    error
}

func newStubRecitationRepo() *stubRecitationRepo {
	return &stubRecitationRepo{
		byStudent: map[uuid.UUID][]model.Recitation{},
	}
}

func (s *stubRecitationRepo) Create(_ context.Context, r *model.Recitation) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	cp := *r
	s.stored = append(s.stored, cp)
	s.byStudent[r.StudentID] = append(s.byStudent[r.StudentID], cp)
	return nil
}

func (s *stubRecitationRepo) BatchCreate(_ context.Context, rs []model.Recitation) error {
	for i := range rs {
		if rs[i].ID == uuid.Nil {
			rs[i].ID = uuid.New()
		}
		s.stored = append(s.stored, rs[i])
		s.byStudent[rs[i].StudentID] = append(s.byStudent[rs[i].StudentID], rs[i])
	}
	return nil
}

func (s *stubRecitationRepo) ListByStudent(_ context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error) {
	s.lastListStudent = studentID
	s.lastListLimit = limit
	s.lastListOffset = offset
	return s.byStudent[studentID], nil
}

func (s *stubRecitationRepo) GetLatestByStudent(_ context.Context, studentID uuid.UUID) (*model.Recitation, error) {
	if s.getLatestErr != nil {
		return nil, s.getLatestErr
	}
	list := s.byStudent[studentID]
	if len(list) == 0 {
		return nil, repo.ErrNotFound
	}
	cp := list[len(list)-1]
	return &cp, nil
}

func validRecordInput() service.RecordInput {
	return service.RecordInput{
		StudentID:   uuid.New(),
		HalaqahID:   uuid.New(),
		Type:        "new_hifz",
		SurahNumber: 2,
		AyahFrom:    1,
		AyahTo:      5,
		Grade:       "mumtaz",
	}
}

func TestRecitationService_Record_HappyPathAndAudit(t *testing.T) {
	recs := newStubRecitationRepo()
	audit := &stubAuditRepo{}
	svc := service.NewRecitationService(recs, audit)

	actor := uuid.New()
	orgID := uuid.New()
	in := validRecordInput()

	rec, err := svc.Record(context.Background(), actor, orgID, in)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if rec.OrganizationID != orgID {
		t.Fatalf("org=%s want %s", rec.OrganizationID, orgID)
	}
	if rec.TeacherID != actor {
		t.Fatalf("teacher=%s want %s", rec.TeacherID, actor)
	}
	if rec.StudentID != in.StudentID {
		t.Fatalf("student mismatch")
	}
	if rec.Type != "new_hifz" || rec.Grade != "mumtaz" {
		t.Fatalf("type/grade mismatch")
	}
	if rec.SurahNumber != 2 || rec.AyahFrom != 1 || rec.AyahTo != 5 {
		t.Fatalf("ayah range mismatch")
	}
	if len(recs.stored) != 1 {
		t.Fatalf("stored=%d want 1", len(recs.stored))
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "record_recitation" {
		t.Fatalf("audit action=%s", audit.logs[0].Action)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("audit actor mismatch")
	}
}

func TestRecitationService_Record_RejectsInvalidType(t *testing.T) {
	svc := service.NewRecitationService(newStubRecitationRepo(), &stubAuditRepo{})
	in := validRecordInput()
	in.Type = "bogus"
	_, err := svc.Record(context.Background(), uuid.New(), uuid.New(), in)
	if !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("err=%v want ErrInvalidRecitationInput", err)
	}
}

func TestRecitationService_Record_RejectsInvalidGrade(t *testing.T) {
	svc := service.NewRecitationService(newStubRecitationRepo(), &stubAuditRepo{})
	in := validRecordInput()
	in.Grade = "no_grade"
	_, err := svc.Record(context.Background(), uuid.New(), uuid.New(), in)
	if !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("err=%v want ErrInvalidRecitationInput", err)
	}
}

func TestRecitationService_Record_RejectsInvalidSurah(t *testing.T) {
	svc := service.NewRecitationService(newStubRecitationRepo(), &stubAuditRepo{})
	in := validRecordInput()
	in.SurahNumber = 200
	_, err := svc.Record(context.Background(), uuid.New(), uuid.New(), in)
	if !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("surah>114: err=%v want ErrInvalidRecitationInput", err)
	}

	in2 := validRecordInput()
	in2.SurahNumber = 0
	if _, err := svc.Record(context.Background(), uuid.New(), uuid.New(), in2); !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("surah=0: err=%v want ErrInvalidRecitationInput", err)
	}
}

func TestRecitationService_Record_RejectsInvalidAyahRange(t *testing.T) {
	svc := service.NewRecitationService(newStubRecitationRepo(), &stubAuditRepo{})
	in := validRecordInput()
	in.AyahFrom = 5
	in.AyahTo = 3
	_, err := svc.Record(context.Background(), uuid.New(), uuid.New(), in)
	if !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("ayah_to<from: err=%v want ErrInvalidRecitationInput", err)
	}

	in2 := validRecordInput()
	in2.AyahFrom = 0
	if _, err := svc.Record(context.Background(), uuid.New(), uuid.New(), in2); !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("ayah_from=0: err=%v want ErrInvalidRecitationInput", err)
	}
}

func TestRecitationService_RecordBatch_HappyPath(t *testing.T) {
	recs := newStubRecitationRepo()
	audit := &stubAuditRepo{}
	svc := service.NewRecitationService(recs, audit)

	actor := uuid.New()
	orgID := uuid.New()
	studentID := uuid.New()
	halaqahID := uuid.New()
	ins := []service.RecordInput{
		{StudentID: studentID, HalaqahID: halaqahID, Type: "new_hifz", SurahNumber: 1, AyahFrom: 1, AyahTo: 7, Grade: "mumtaz"},
		{StudentID: studentID, HalaqahID: halaqahID, Type: "near_review", SurahNumber: 2, AyahFrom: 1, AyahTo: 5, Grade: "jayyid"},
		{StudentID: studentID, HalaqahID: halaqahID, Type: "far_review", SurahNumber: 3, AyahFrom: 10, AyahTo: 20, Grade: "maqbul"},
	}
	out, err := svc.RecordBatch(context.Background(), actor, orgID, ins)
	if err != nil {
		t.Fatalf("RecordBatch: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("returned=%d want 3", len(out))
	}
	if len(recs.stored) != 3 {
		t.Fatalf("stored=%d want 3", len(recs.stored))
	}
	for i, r := range out {
		if r.OrganizationID != orgID {
			t.Fatalf("idx=%d org mismatch", i)
		}
		if r.TeacherID != actor {
			t.Fatalf("idx=%d teacher mismatch", i)
		}
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "record_recitation_batch" {
		t.Fatalf("audit action=%s", audit.logs[0].Action)
	}
}

func TestRecitationService_RecordBatch_ValidatesEachInput(t *testing.T) {
	recs := newStubRecitationRepo()
	audit := &stubAuditRepo{}
	svc := service.NewRecitationService(recs, audit)

	ins := []service.RecordInput{
		validRecordInput(),
		// invalid: bad grade
		{StudentID: uuid.New(), HalaqahID: uuid.New(), Type: "new_hifz", SurahNumber: 1, AyahFrom: 1, AyahTo: 2, Grade: "nope"},
	}
	_, err := svc.RecordBatch(context.Background(), uuid.New(), uuid.New(), ins)
	if !errors.Is(err, service.ErrInvalidRecitationInput) {
		t.Fatalf("err=%v want ErrInvalidRecitationInput", err)
	}
	if len(recs.stored) != 0 {
		t.Fatalf("stored=%d want 0 (no partial inserts)", len(recs.stored))
	}
	if len(audit.logs) != 0 {
		t.Fatalf("audit on failed batch=%d want 0", len(audit.logs))
	}
}

func TestRecitationService_ListForStudent_ClampsLimit(t *testing.T) {
	recs := newStubRecitationRepo()
	svc := service.NewRecitationService(recs, &stubAuditRepo{})
	sid := uuid.New()
	if _, err := svc.ListForStudent(context.Background(), sid, 0, 0); err != nil {
		t.Fatalf("ListForStudent(0): %v", err)
	}
	if recs.lastListLimit != 50 {
		t.Fatalf("limit=0 → %d want 50", recs.lastListLimit)
	}
	if _, err := svc.ListForStudent(context.Background(), sid, 500, 10); err != nil {
		t.Fatalf("ListForStudent(500): %v", err)
	}
	if recs.lastListLimit != 50 {
		t.Fatalf("limit=500 → %d want 50 (clamped)", recs.lastListLimit)
	}
	if recs.lastListOffset != 10 {
		t.Fatalf("offset=%d want 10", recs.lastListOffset)
	}
	if recs.lastListStudent != sid {
		t.Fatalf("student id forwarded wrong")
	}
}

func TestRecitationService_GetLatest_PassThrough(t *testing.T) {
	recs := newStubRecitationRepo()
	svc := service.NewRecitationService(recs, &stubAuditRepo{})

	sid := uuid.New()
	// not found case
	if _, err := svc.GetLatest(context.Background(), sid); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("empty: err=%v want ErrNotFound", err)
	}

	// seed one
	recs.byStudent[sid] = []model.Recitation{{ID: uuid.New(), StudentID: sid, Grade: "mumtaz"}}
	got, err := svc.GetLatest(context.Background(), sid)
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if got.StudentID != sid {
		t.Fatalf("student id mismatch")
	}
}
