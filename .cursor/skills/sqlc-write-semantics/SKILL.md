---
name: sqlc-write-semantics
description: Guides correct field selection in SQL write operations. Use when writing any INSERT, UPDATE, or upsert query in apps/api/queries/ — ensures only semantically relevant fields are written per operation type.
---

# sqlc — write semantics

Before writing an INSERT or UPDATE query, identify the operation type and include **only the fields that operation semantically changes**.

## Field matrix

| Field | INSERT | UPDATE | soft-delete |
|---|---|---|---|
| `id` | ✅ | — (it's the key) | — |
| `created_at` | ✅ | ❌ never | ❌ never |
| `updated_at` | ✅ | ✅ | ✅ |
| `deleted_at` | absent | absent | ✅ NOW() |
| `sync_cursor` | ✅ | ✅ | ✅ |
| payload fields | ✅ all | only changed ones | ❌ |

## Upsert — prefer explicit conflict handling

`INSERT ... ON CONFLICT DO UPDATE` with all fields silently overwrites `created_at` on conflict.

**Preferred pattern:**

```sql
-- name: UpsertProject :one
INSERT INTO projects (id, workspace_id, name, created_at, updated_at, sync_cursor)
VALUES (@id, @workspace_id, @name, @created_at, @updated_at, @sync_cursor)
ON CONFLICT (id) DO UPDATE SET
    name         = EXCLUDED.name,
    updated_at   = EXCLUDED.updated_at,
    sync_cursor  = EXCLUDED.sync_cursor
RETURNING *;
```

`created_at` is never in the `DO UPDATE SET` clause.

## Checklist before writing a query

- [ ] `created_at` is absent from every UPDATE and soft-delete query.
- [ ] `deleted_at` is set only in soft-delete queries, absent everywhere else.
- [ ] `updated_at` is `NOW()` or a passed timestamp on every mutation.
- [ ] Upsert does not silently overwrite `created_at` via `ON CONFLICT DO UPDATE`.
