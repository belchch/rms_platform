-- name: GetWallByID :one
SELECT * FROM walls WHERE id = $1 LIMIT 1;

-- name: UpsertWall :one
INSERT INTO walls (id, room_id, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET
    updated_at = EXCLUDED.updated_at,
    deleted_at = NULL
RETURNING *;

-- name: SoftDeleteWall :one
UPDATE walls SET deleted_at = now(), updated_at = $2 WHERE id = $1 RETURNING *;

-- name: ListWallsSince :many
SELECT walls.* FROM walls
JOIN rooms ON walls.room_id = rooms.id
JOIN plans ON rooms.plan_id = plans.id
JOIN projects ON plans.project_id = projects.id
WHERE projects.workspace_id = $1 AND walls.sync_cursor > $2
ORDER BY walls.sync_cursor;
