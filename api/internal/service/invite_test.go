// api/internal/service/invite_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

// --- stubs ---

type stubInviteRepo struct {
	byToken map[string]*model.Invite
	created *model.Invite
	usedID  uuid.UUID
}

func (s *stubInviteRepo) Create(_ context.Context, i *model.Invite) error {
	i.ID = uuid.New()
	if s.byToken == nil {
		s.byToken = map[string]*model.Invite{}
	}
	s.byToken[i.Token] = i
	s.created = i
	return nil
}
func (s *stubInviteRepo) GetByToken(_ context.Context, t string) (*model.Invite, error) {
	if i, ok := s.byToken[t]; ok {
		return i, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubInviteRepo) MarkUsed(_ context.Context, id uuid.UUID) error {
	s.usedID = id
	if s.byToken != nil {
		for _, inv := range s.byToken {
			if inv.ID == id {
				now := time.Now()
				inv.UsedAt = &now
			}
		}
	}
	return nil
}

type stubUserRepoFull struct {
	byEmail map[string]*model.User
	created *model.User
}

func (s *stubUserRepoFull) GetByEmailGlobal(_ context.Context, e string) (*model.User, error) {
	if u, ok := s.byEmail[e]; ok {
		return u, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubUserRepoFull) Create(_ context.Context, u *model.User) error {
	u.ID = uuid.New()
	if s.byEmail == nil {
		s.byEmail = map[string]*model.User{}
	}
	if u.Email != nil {
		s.byEmail[*u.Email] = u
	}
	s.created = u
	return nil
}
func (s *stubUserRepoFull) Update(_ context.Context, _ *model.User) error { return nil }

// --- tests ---

func TestInviteService_Generate_HappyPath(t *testing.T) {
	invs := &stubInviteRepo{}
	users := &stubUserRepoFull{}
	otps := &stubOtpRepo{}
	es := email.NewLogSender()
	audit := &stubAuditRepo{}

	svc := service.NewInviteService(invs, users, otps, es, audit)

	orgID := uuid.New()
	creator := uuid.New()
	inv, err := svc.GenerateForCenterAdmin(context.Background(), creator, orgID)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if inv.Token == "" {
		t.Fatal("empty token")
	}
	if invs.created != inv {
		t.Fatal("repo not called")
	}
	if inv.Role != "center_admin" {
		t.Fatalf("role=%s", inv.Role)
	}
	if inv.OrganizationID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
}

func TestInviteService_GenerateForTeacher_HappyPath(t *testing.T) {
	invs := &stubInviteRepo{}
	users := &stubUserRepoFull{}
	otps := &stubOtpRepo{}
	es := email.NewLogSender()
	audit := &stubAuditRepo{}

	svc := service.NewInviteService(invs, users, otps, es, audit)

	orgID := uuid.New()
	creator := uuid.New()
	inv, err := svc.GenerateForTeacher(context.Background(), creator, orgID)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if inv.Token == "" {
		t.Fatal("empty token")
	}
	if invs.created != inv {
		t.Fatal("repo not called")
	}
	if inv.Role != "teacher" {
		t.Fatalf("role=%s", inv.Role)
	}
	if inv.OrganizationID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
}

func TestInviteService_Accept_HappyPath(t *testing.T) {
	orgID := uuid.New()
	creator := uuid.New()
	now := time.Now()
	inv := &model.Invite{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Role:           "center_admin",
		Token:          "tok-1",
		ExpiresAt:      now.Add(time.Hour),
		CreatedBy:      creator,
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-1": inv}}
	users := &stubUserRepoFull{byEmail: map[string]*model.User{}}
	otps := &stubOtpRepo{}
	es := email.NewLogSender()
	audit := &stubAuditRepo{}

	svc := service.NewInviteService(invs, users, otps, es, audit)

	if err := svc.AcceptInvite(context.Background(), "tok-1", "newadmin@x.com", "Center Admin"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// User pre-created with status=pending and role from invite.
	if users.created == nil {
		t.Fatal("user not created")
	}
	if users.created.Status != "pending" {
		t.Fatalf("status=%s want pending", users.created.Status)
	}
	if users.created.Role != "center_admin" {
		t.Fatalf("role=%s", users.created.Role)
	}
	if users.created.OrganizationID == nil || *users.created.OrganizationID != orgID {
		t.Fatal("org_id mismatch")
	}
	// OTP stored + email sent.
	if otps.created == nil {
		t.Fatal("OTP not stored")
	}
	if es.Last().To != "newadmin@x.com" {
		t.Fatalf("email To: %s", es.Last().To)
	}
	// Invite marked used.
	if inv.UsedAt == nil {
		t.Fatal("invite not marked used")
	}
}

func TestInviteService_Accept_BadToken(t *testing.T) {
	svc := service.NewInviteService(&stubInviteRepo{}, &stubUserRepoFull{}, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})
	err := svc.AcceptInvite(context.Background(), "ghost", "x@y", "X")
	if !errors.Is(err, service.ErrInvalidInvite) {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
}

func TestInviteService_Accept_ExpiredInvite(t *testing.T) {
	inv := &model.Invite{
		ID: uuid.New(), OrganizationID: uuid.New(), Role: "center_admin",
		Token: "tok-x", ExpiresAt: time.Now().Add(-time.Hour), CreatedBy: uuid.New(),
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-x": inv}}
	svc := service.NewInviteService(invs, &stubUserRepoFull{}, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})

	if err := svc.AcceptInvite(context.Background(), "tok-x", "x@y", "X"); !errors.Is(err, service.ErrInvalidInvite) {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
}

func TestInviteService_Accept_AlreadyUsed(t *testing.T) {
	used := time.Now().Add(-time.Hour)
	inv := &model.Invite{
		ID: uuid.New(), OrganizationID: uuid.New(), Role: "center_admin",
		Token: "tok-u", ExpiresAt: time.Now().Add(time.Hour), UsedAt: &used, CreatedBy: uuid.New(),
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-u": inv}}
	svc := service.NewInviteService(invs, &stubUserRepoFull{}, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})
	if err := svc.AcceptInvite(context.Background(), "tok-u", "x@y", "X"); !errors.Is(err, service.ErrInvalidInvite) {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
}

func TestInviteService_Accept_EmailAlreadyTaken(t *testing.T) {
	inv := &model.Invite{
		ID: uuid.New(), OrganizationID: uuid.New(), Role: "center_admin",
		Token: "tok-e", ExpiresAt: time.Now().Add(time.Hour), CreatedBy: uuid.New(),
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-e": inv}}
	users := &stubUserRepoFull{byEmail: map[string]*model.User{
		"taken@x": {ID: uuid.New(), Email: ptrStrSvc("taken@x")},
	}}
	svc := service.NewInviteService(invs, users, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})
	if err := svc.AcceptInvite(context.Background(), "tok-e", "taken@x", "X"); !errors.Is(err, service.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func ptrStrSvc(s string) *string { return &s }

// auth import is used only to pull GenerateOTP for symmetry with auth_test
var _ = auth.GenerateOTP
