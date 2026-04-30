-- name: GetPhotoByID :one
SELECT * FROM photos WHERE id = $1 LIMIT 1;

-- name: UpsertPhoto :one
INSERT INTO photos (id, photoable_id, name, caption, taken_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
    name       = EXCLUDED.name,
    caption    = EXCLUDED.caption,
    taken_at   = EXCLUDED.taken_at,
    updated_at = EXCLUDED.updated_at,
    deleted_at = NULL
RETURNING *;

-- name: SetPhotoRemoteURL :exec
UPDATE photos SET remote_url = $2 WHERE id = $1;

-- name: SoftDeletePhoto :one
UPDATE photos SET deleted_at = now(), updated_at = $2 WHERE id = $1 RETURNING *;

-- name: ListPhotosSince :many
SELECT p.* FROM photos p
JOIN photoables pa ON p.photoable_id = pa.id
JOIN projects proj ON pa.owner_type = 'project' AND pa.owner_id = proj.id
WHERE proj.workspace_id = $1 AND p.sync_cursor > $2
UNION ALL
SELECT p.* FROM photos p
JOIN photoables pa ON p.photoable_id = pa.id
JOIN rooms r ON pa.owner_type = 'room' AND pa.owner_id = r.id
JOIN plans pl ON r.plan_id = pl.id
JOIN projects proj ON pl.project_id = proj.id
WHERE proj.workspace_id = $1 AND p.sync_cursor > $2
UNION ALL
SELECT p.* FROM photos p
JOIN photoables pa ON p.photoable_id = pa.id
JOIN walls w ON pa.owner_type = 'wall' AND pa.owner_id = w.id
JOIN rooms r ON w.room_id = r.id
JOIN plans pl ON r.plan_id = pl.id
JOIN projects proj ON pl.project_id = proj.id
WHERE proj.workspace_id = $1 AND p.sync_cursor > $2
ORDER BY sync_cursor;
