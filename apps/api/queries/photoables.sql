-- name: GetPhotoableByID :one
SELECT * FROM photoables WHERE id = $1 LIMIT 1;

-- name: GetPhotoableByOwner :one
SELECT * FROM photoables WHERE owner_type = $1 AND owner_id = $2 LIMIT 1;

-- name: CreatePhotoable :one
INSERT INTO photoables (id, owner_type, owner_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpsertPhotoableByOwner :one
INSERT INTO photoables (id, owner_type, owner_id)
VALUES ($1, $2, $3)
ON CONFLICT (owner_type, owner_id) DO UPDATE SET owner_type = EXCLUDED.owner_type
RETURNING *;
