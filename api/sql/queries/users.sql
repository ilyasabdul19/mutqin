-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 AND organization_id = $2;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND organization_id = $2;

-- name: GetUserByEmailAdmin :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByIDAdmin :one
SELECT * FROM users WHERE id = $1;
