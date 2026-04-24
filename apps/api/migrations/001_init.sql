-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    owner_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE projects (
    id            TEXT PRIMARY KEY,
    workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    address       TEXT,
    description   TEXT,
    is_archived   BOOLEAN NOT NULL DEFAULT false,
    is_favourite  BOOLEAN NOT NULL DEFAULT false,
    cover_photo_id TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    sync_cursor   BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE plans (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    payload_json JSONB,
    thumbnail_path TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    sync_cursor BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE rooms (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE walls (
    id          TEXT PRIMARY KEY,
    room_id     TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE photoables (
    id          TEXT PRIMARY KEY,
    owner_type  TEXT NOT NULL,
    owner_id    TEXT NOT NULL,
    UNIQUE (owner_type, owner_id)
);

CREATE TABLE photos (
    id            TEXT PRIMARY KEY,
    photoable_id  TEXT NOT NULL REFERENCES photoables(id) ON DELETE CASCADE,
    local_path    TEXT,
    remote_url    TEXT,
    name          TEXT,
    caption       TEXT,
    taken_at      TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    sync_cursor   BIGINT NOT NULL DEFAULT 0
);

ALTER TABLE projects
    ADD CONSTRAINT fk_projects_cover_photo
    FOREIGN KEY (cover_photo_id) REFERENCES photos(id) ON DELETE SET NULL;

CREATE INDEX idx_projects_workspace_id ON projects(workspace_id);
CREATE INDEX idx_projects_updated_at ON projects(updated_at);
CREATE INDEX idx_plans_project_id ON plans(project_id);
CREATE INDEX idx_photos_photoable_id ON photos(photoable_id);
CREATE INDEX idx_photoables_owner ON photoables(owner_type, owner_id);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_cover_photo;
DROP TABLE IF EXISTS photos;
DROP TABLE IF EXISTS photoables;
DROP TABLE IF EXISTS walls;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

-- +goose StatementEnd
