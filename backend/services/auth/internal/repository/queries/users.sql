-- name: GetUserByLinkedInSub :one
SELECT * FROM users WHERE linkedin_sub = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (linkedin_sub, full_name, profile_photo_url, headline, trust_level)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
