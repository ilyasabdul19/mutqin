// api/internal/handler/api/platform.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

type OrgService interface {
	Create(ctx context.Context, actor uuid.UUID, in service.CreateOrgInput) (*model.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
	List(ctx context.Context, limit, offset int) ([]model.Organization, error)
	UpdateLanding(ctx context.Context, actor, orgID uuid.UUID, in service.UpdateLandingInput) error
}

type InviteIssuer interface {
	GenerateForCenterAdmin(ctx context.Context, actor, orgID uuid.UUID) (*model.Invite, error)
}

type PlatformHandler struct {
	orgs    OrgService
	invites InviteIssuer
}

func NewPlatformHandler(orgs OrgService, invites InviteIssuer) *PlatformHandler {
	return &PlatformHandler{orgs: orgs, invites: invites}
}

type createOrgBody struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Country     string  `json:"country"`
	City        *string `json:"city"`
	Description *string `json:"description"`
	Tier        string  `json:"tier"`
}

func (h *PlatformHandler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	var b createOrgBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	if strings.TrimSpace(b.Name) == "" || strings.TrimSpace(b.Slug) == "" || strings.TrimSpace(b.Country) == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "name, slug, country required")
		return
	}
	o, err := h.orgs.Create(r.Context(), id.UserID, service.CreateOrgInput{
		Name: b.Name, Slug: b.Slug, Country: b.Country, City: b.City, Description: b.Description, Tier: b.Tier,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not create organization")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": o})
}

func (h *PlatformHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, err := h.orgs.List(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, out, len(out))
}

func (h *PlatformHandler) GetOrgBySlug(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	slug := chi.URLParam(r, "slug")
	o, err := h.orgs.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "organization not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "lookup failed")
		return
	}
	response.Success(w, o)
}

func (h *PlatformHandler) GenerateInvite(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid org id")
		return
	}
	inv, err := h.invites.GenerateForCenterAdmin(r.Context(), id.UserID, orgID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not generate invite")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
		"token":      inv.Token,
		"expires_at": inv.ExpiresAt,
	}})
}

// updateLandingBody is what PATCH /api/v1/organizations/me/landing accepts.
// Schedule is a raw JSON value so callers can pass arbitrary structured
// shapes (object, array, etc.) — it's stored as jsonb downstream.
type updateLandingBody struct {
	Description *string         `json:"description"`
	LogoURL     *string         `json:"logo_url"`
	City        *string         `json:"city"`
	Schedule    json.RawMessage `json:"schedule"`
}

// UpdateLanding patches the landing-page-visible fields on the caller's
// own organization. The center_admin's orgID is read from the JWT-derived
// Identity in ctx, so the endpoint is self-scoped — center_admin cannot
// edit another center's landing page through this route.
func (h *PlatformHandler) UpdateLanding(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	var b updateLandingBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	in := service.UpdateLandingInput{
		Description: b.Description,
		LogoURL:     b.LogoURL,
		City:        b.City,
	}
	if len(b.Schedule) > 0 {
		in.Schedule = []byte(b.Schedule)
	}
	if err := h.orgs.UpdateLanding(r.Context(), id.UserID, *id.OrgID, in); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "organization not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not update landing")
		return
	}
	response.Success(w, map[string]string{"status": "updated"})
}

// SetURLParamForTest is a thin testing helper — adds a Chi URL parameter to a
// request context so handlers that read chi.URLParam don't need a full router.
func SetURLParamForTest(r *http.Request, key, value string) {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	*r = *r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
