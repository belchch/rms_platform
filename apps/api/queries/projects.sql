-- name: GetProjectByID :one
SELECT * FROM projects WHERE id = $1 LIMIT 1;

-- name: UpsertProject :one
INSERT INTO projects (id, workspace_id, name, address, description, is_archived, is_favourite, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    name         = EXCLUDED.name,
    address      = EXCLUDED.address,
    description  = EXCLUDED.description,
    is_archived  = EXCLUDED.is_archived,
    is_favourite = EXCLUDED.is_favourite,
    updated_at   = EXCLUDED.updated_at
RETURNING *;

-- name: SoftDeleteProject :one
UPDATE projects SET deleted_at = now() WHERE id = $1 RETURNING *;

-- name: ListProjectsSince :many
SELECT * FROM projects
WHERE workspace_id = $1 AND sync_cursor > $2
ORDER BY sync_cursor;
