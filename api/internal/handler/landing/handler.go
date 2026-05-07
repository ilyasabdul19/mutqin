// api/internal/handler/landing/handler.go
package landing

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

// OrgLookup resolves a slug to an organization. The landing handler uses the
// admin handle so RLS doesn't apply (the slug is a public lookup key).
type OrgLookup interface {
	GetBySlugAdmin(ctx context.Context, slug string) (*model.Organization, error)
}

// Handler bundles the dependencies for landing routes.
type Handler struct {
	orgs   OrgLookup
	regs   *repo.RegistrationRepo
	tmpls  map[string]*template.Template
	static fs.FS
	logger *slog.Logger
}

// New constructs a Handler. Each page template is parsed together with the
// shared layout so executing layout against that page-specific set picks up
// the page's content/title block overrides.
func New(orgs OrgLookup, regs *repo.RegistrationRepo, logger *slog.Logger) (*Handler, error) {
	pages := []string{
		"center.html.tmpl",
		"register.html.tmpl",
		"success.html.tmpl",
		"not_found.html.tmpl",
	}
	tmpls := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		t, err := template.ParseFS(Templates(), "layout.html.tmpl", page)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		tmpls[page] = t
	}
	return &Handler{
		orgs:   orgs,
		regs:   regs,
		tmpls:  tmpls,
		static: Static(),
		logger: logger,
	}, nil
}

// Routes returns a Chi mux mounted at the root of the landing domain.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/static/*", h.serveStatic)
	r.Get("/{slug}", h.center)
	r.Get("/{slug}/register", h.registerForm)
	r.Post("/{slug}/register", h.submitRegistration)
	r.NotFound(h.notFound)
	return r
}

func (h *Handler) render(w http.ResponseWriter, page string, status int, data any) {
	t, ok := h.tmpls[page]
	if !ok {
		http.Error(w, "unknown template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "layout.html.tmpl", data); err != nil {
		h.logger.Error("render", "page", page, "error", err)
	}
}

func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	stripped := strings.TrimPrefix(r.URL.Path, "/static/")
	http.ServeFileFS(w, r, h.static, stripped)
}

func (h *Handler) resolveOrg(ctx context.Context, slug string) (*model.Organization, context.Context, bool) {
	org, err := h.orgs.GetBySlugAdmin(ctx, slug)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ctx, false
		}
		h.logger.Error("org lookup", "slug", slug, "error", err)
		return nil, ctx, false
	}
	return org, tenant.With(ctx, org.ID), true
}

func (h *Handler) center(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, _, ok := h.resolveOrg(r.Context(), slug)
	if !ok {
		h.notFound(w, r)
		return
	}
	h.render(w, "center.html.tmpl", http.StatusOK, map[string]any{"Org": org})
}

func (h *Handler) registerForm(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, _, ok := h.resolveOrg(r.Context(), slug)
	if !ok {
		h.notFound(w, r)
		return
	}
	h.render(w, "register.html.tmpl", http.StatusOK, map[string]any{"Org": org})
}

func (h *Handler) submitRegistration(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, ctx, ok := h.resolveOrg(r.Context(), slug)
	if !ok {
		h.notFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	childName := strings.TrimSpace(r.PostForm.Get("child_name"))
	if childName == "" {
		h.render(w, "register.html.tmpl", http.StatusBadRequest, map[string]any{
			"Org":   org,
			"Error": "Child's name is required.",
		})
		return
	}
	reg := &model.Registration{
		OrganizationID: org.ID,
		ChildName:      childName,
	}
	if v := strings.TrimSpace(r.PostForm.Get("child_age")); v != "" {
		if age, err := strconv.Atoi(v); err == nil && age > 0 && age < 100 {
			reg.ChildAge = &age
		}
	}
	if v := strings.TrimSpace(r.PostForm.Get("parent_phone")); v != "" {
		reg.ParentPhone = &v
	}
	if v := strings.TrimSpace(r.PostForm.Get("parent_email")); v != "" {
		reg.ParentEmail = &v
	}
	if v := strings.TrimSpace(r.PostForm.Get("hifz_level")); v != "" {
		reg.HifzLevel = &v
	}
	if err := h.regs.Create(ctx, reg); err != nil {
		h.logger.Error("registration create", "slug", slug, "error", err)
		http.Error(w, "could not save registration", http.StatusInternalServerError)
		return
	}
	h.render(w, "success.html.tmpl", http.StatusOK, map[string]any{
		"Org":       org,
		"ChildName": childName,
	})
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, "not_found.html.tmpl", http.StatusNotFound, nil)
}
