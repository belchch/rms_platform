-- name: GetWorkspaceByID :one
SELECT * FROM workspaces WHERE id = $1 LIMIT 1;

-- name: GetWorkspaceByOwnerID :one
SELECT * FROM workspaces WHERE owner_id = $1 LIMIT 1;

-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, owner_id)
VALUES ($1, $2, $3)
RETURNING *;
