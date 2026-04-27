-- name: GetRoomByID :one
SELECT * FROM rooms WHERE id = $1 LIMIT 1;

-- name: UpsertRoom :one
INSERT INTO rooms (id, plan_id, name, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
    name       = EXCLUDED.name,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: SoftDeleteRoom :one
UPDATE rooms SET deleted_at = now() WHERE id = $1 RETURNING *;

-- name: ListRoomsSince :many
SELECT rooms.* FROM rooms
JOIN plans ON rooms.plan_id = plans.id
JOIN projects ON plans.project_id = projects.id
WHERE projects.workspace_id = $1 AND rooms.sync_cursor > $2
ORDER BY rooms.sync_cursor;
