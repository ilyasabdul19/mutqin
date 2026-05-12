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

// SetURLParamForTest is a thin testing helper — adds a Chi URL parameter to a
// request context so handlers that read chi.URLParam don't need a full router.
func SetURLParamForTest(r *http.Request, key, value string) {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	*r = *r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
