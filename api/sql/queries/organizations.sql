-- name: CreateOrganization :one
INSERT INTO organizations (name, slug, city, country, tier, status, logo_url, description, schedule)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;

-- name: ListOrganizations :many
SELECT * FROM organizations ORDER BY created_at DESC LIMIT $1 OFFSET $2;
