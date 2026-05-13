// api/cmd/server/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/handler"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type orgLookupAdapter struct{ r *repo.OrganizationRepo }

func (a orgLookupAdapter) GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	org, err := a.r.GetBySlugAdmin(ctx, slug)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	migrateUp := flag.Bool("migrate-up", false, "apply pending migrations and exit")
	migrateDown := flag.Bool("migrate-down", false, "roll back the most recent migration group and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Admin/superuser handle — used for migrations and cross-tenant lookups
	// (e.g. resolving a slug to an org UUID before the tenant ctx is set).
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect admin database", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	switch {
	case *migrateUp:
		if err := migrate.Up(ctx, adminDB); err != nil {
			slog.Error("migrate up", "error", err)
			os.Exit(1)
		}
		return
	case *migrateDown:
		if err := migrate.Down(ctx, adminDB); err != nil {
			slog.Error("migrate down", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := migrate.Up(ctx, adminDB); err != nil {
		slog.Error("apply migrations on startup", "error", err)
		os.Exit(1)
	}

	// App handle — non-superuser, RLS-subject. Per-model hooks via
	// model.TenantScoped inject organization_id filters when ctx has a tenant;
	// Postgres RLS enforces at the storage layer regardless.
	appDB, err := db.NewDB(ctx, cfg.AppDatabaseURL, false)
	if err != nil {
		slog.Error("connect app database", "error", err)
		os.Exit(1)
	}
	defer appDB.Close()
	slog.Info("connected to database",
		"admin_dsn_redacted", redactDSN(cfg.DatabaseURL),
		"app_dsn_redacted", redactDSN(cfg.AppDatabaseURL))

	orgLookup := orgLookupAdapter{r: repo.NewOrganizationRepo(adminDB)}

	// Auth wiring.
	jwtIssuer := auth.NewIssuer([]byte(cfg.JWTSecret), cfg.JWTIssuer)
	jwtVerifier := auth.NewVerifier([]byte(cfg.JWTSecret), cfg.JWTIssuer)
	emailSender := email.NewLogSender() // swap to SMTP when RESEND_API_KEY lands
	authSvc := service.NewAuthService(
		repo.NewOtpRepo(adminDB),
		repo.NewUserRepo(adminDB),
		emailSender,
		jwtIssuer,
		24*time.Hour,
	)
	authH := apihandler.NewAuthHandler(authSvc)

	auditRepo := repo.NewAuditLogRepo(adminDB)
	orgSvc := service.NewOrganizationService(repo.NewOrganizationRepo(adminDB), auditRepo)
	inviteSvc := service.NewInviteService(
		repo.NewInviteRepo(adminDB),
		repo.NewUserRepo(adminDB),
		repo.NewOtpRepo(adminDB),
		emailSender,
		auditRepo,
	)
	platformH := apihandler.NewPlatformHandler(orgSvc, inviteSvc)
	inviteAcceptH := apihandler.NewInviteAcceptHandler(inviteSvc)

	halaqahRepo := repo.NewHalaqahRepo(appDB)
	halaqahSvc := service.NewHalaqahService(halaqahRepo, auditRepo)
	studentSvc := service.NewStudentService(repo.NewStudentRepo(appDB), halaqahRepo, auditRepo)
	attendanceSvc := service.NewAttendanceService(repo.NewAttendanceRepo(appDB), auditRepo)
	halaqatH := apihandler.NewHalaqatHandler(halaqahSvc)
	studentsH := apihandler.NewStudentsHandler(studentSvc)
	teachersH := apihandler.NewTeachersHandler(inviteSvc)
	attendanceH := apihandler.NewAttendanceHandler(attendanceSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS)
	r.Use(middleware.Tenant(orgLookup, cfg.BaseHost))
	r.Use(middleware.Auth(jwtVerifier))
	r.Use(middleware.AuthAsTenant)
	r.Use(middleware.RLSContext(middleware.NewBunRunner(appDB)))

	r.Get("/api/v1/health", handler.Health())
	r.Post("/api/v1/auth/otp/request", authH.RequestOTP)
	r.Post("/api/v1/auth/otp/verify", authH.VerifyOTP)
	r.Post("/api/v1/auth/invite/accept", inviteAcceptH.Accept)

	// Super-admin platform routes.
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.Role("super_admin"))
		pr.Post("/api/v1/organizations", platformH.CreateOrg)
		pr.Get("/api/v1/organizations", platformH.ListOrgs)
		pr.Get("/api/v1/organizations/{slug}", platformH.GetOrgBySlug)
		pr.Post("/api/v1/organizations/{id}/invite", platformH.GenerateInvite)
	})

	// Center-admin routes (super_admin can also access).
	r.Group(func(ca chi.Router) {
		ca.Use(middleware.Role("center_admin", "super_admin"))
		ca.Post("/api/v1/halaqat", halaqatH.Create)
		ca.Get("/api/v1/halaqat", halaqatH.List)
		ca.Get("/api/v1/halaqat/{id}", halaqatH.Get)
		ca.Patch("/api/v1/halaqat/{id}", halaqatH.Update)
		ca.Post("/api/v1/halaqat/{id}/students", studentsH.Enroll)
		ca.Post("/api/v1/students/{id}/transfer", studentsH.Transfer)
		ca.Post("/api/v1/teachers/invite", teachersH.GenerateInvite)
		ca.Get("/api/v1/students/{id}/attendance", attendanceH.ListByStudent)
		ca.Get("/api/v1/attendance", attendanceH.ListByOrg)
	})

	// Teacher + admin routes (read-only listing + attendance marking).
	r.Group(func(tr chi.Router) {
		tr.Use(middleware.Role("teacher", "center_admin", "super_admin"))
		tr.Get("/api/v1/halaqat/{id}/students", studentsH.ListByHalaqah)
		tr.Post("/api/v1/halaqat/{id}/attendance", attendanceH.MarkBatch)
		tr.Get("/api/v1/halaqat/{id}/attendance", attendanceH.ListByHalaqahDate)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// redactDSN returns the input DSN with the password component blanked.
func redactDSN(s string) string {
	at := -1
	for i := range s {
		if s[i] == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return s
	}
	count := 0
	colon := -1
	for i := 0; i < at; i++ {
		if s[i] == ':' {
			count++
			if count == 2 {
				colon = i
				break
			}
		}
	}
	if colon < 0 {
		return s
	}
	return s[:colon+1] + "***" + s[at:]
}
