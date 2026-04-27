-- name: GetPlanByID :one
SELECT * FROM plans WHERE id = $1 LIMIT 1;

-- name: UpsertPlan :one
INSERT INTO plans (id, project_id, name, payload_json, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
    name         = EXCLUDED.name,
    payload_json = EXCLUDED.payload_json,
    updated_at   = EXCLUDED.updated_at
RETURNING *;

-- name: SoftDeletePlan :one
UPDATE plans SET deleted_at = now() WHERE id = $1 RETURNING *;

-- name: ListPlansSince :many
SELECT plans.* FROM plans
JOIN projects ON plans.project_id = projects.id
WHERE projects.workspace_id = $1 AND plans.sync_cursor > $2
ORDER BY plans.sync_cursor;
