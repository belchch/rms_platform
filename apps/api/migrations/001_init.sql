-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE SEQUENCE sync_cursor_seq;

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
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    owner_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    sync_cursor BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq')
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
    sync_cursor   BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq')
);

CREATE TABLE plans (
    id           TEXT PRIMARY KEY,
    project_id   TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    name         TEXT NOT NULL,
    payload_json JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_cursor  BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq')
);

CREATE TABLE rooms (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    name        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    sync_cursor BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq')
);

CREATE TABLE walls (
    id          TEXT PRIMARY KEY,
    room_id     TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    sync_cursor BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq')
);

CREATE TABLE photoables (
    id         TEXT PRIMARY KEY,
    owner_type TEXT NOT NULL,
    owner_id   TEXT NOT NULL,
    UNIQUE (owner_type, owner_id)
);

CREATE TABLE photos (
    id           TEXT PRIMARY KEY,
    photoable_id TEXT NOT NULL REFERENCES photoables(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    remote_url   TEXT,
    content_type TEXT NOT NULL DEFAULT '',
    name         TEXT,
    caption      TEXT,
    taken_at     TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_cursor  BIGINT NOT NULL DEFAULT nextval('sync_cursor_seq')
);

ALTER TABLE projects
    ADD CONSTRAINT fk_projects_cover_photo
    FOREIGN KEY (cover_photo_id) REFERENCES photos(id) ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;

CREATE OR REPLACE FUNCTION bump_sync_cursor() RETURNS TRIGGER AS $$
BEGIN
    NEW.sync_cursor := nextval('sync_cursor_seq');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_workspaces_sync_cursor
    BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();

CREATE TRIGGER trg_projects_sync_cursor
    BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();

CREATE TRIGGER trg_plans_sync_cursor
    BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();

CREATE TRIGGER trg_rooms_sync_cursor
    BEFORE UPDATE ON rooms
    FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();

CREATE TRIGGER trg_walls_sync_cursor
    BEFORE UPDATE ON walls
    FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();

CREATE TRIGGER trg_photos_sync_cursor
    BEFORE UPDATE ON photos
    FOR EACH ROW EXECUTE FUNCTION bump_sync_cursor();

CREATE INDEX idx_workspaces_sync_cursor ON workspaces(sync_cursor);
CREATE INDEX idx_projects_workspace_id ON projects(workspace_id);
CREATE INDEX idx_projects_workspace_sync ON projects(workspace_id, sync_cursor);
CREATE INDEX idx_projects_sync_cursor ON projects(sync_cursor);
CREATE INDEX idx_plans_project_id ON plans(project_id);
CREATE INDEX idx_plans_sync_cursor ON plans(sync_cursor);
CREATE INDEX idx_rooms_plan_id ON rooms(plan_id);
CREATE INDEX idx_rooms_sync_cursor ON rooms(sync_cursor);
CREATE INDEX idx_walls_room_id ON walls(room_id);
CREATE INDEX idx_walls_sync_cursor ON walls(sync_cursor);
CREATE INDEX idx_photos_photoable_id ON photos(photoable_id);
CREATE INDEX idx_photos_sync_cursor ON photos(sync_cursor);
CREATE INDEX idx_photoables_owner ON photoables(owner_type, owner_id);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

INSERT INTO users (id, name, email, password_hash)
VALUES (
    'a0000000-0000-4000-8000-000000000001',
    'Demo',
    'demo@rms.local',
    '$2a$10$PDj6be7DFXy8QNaixujvouQZIyvyMMODOlC3r3qArkOWU83E990Ti'
);

INSERT INTO workspaces (id, name, owner_id)
VALUES (
    'b0000000-0000-4000-8000-000000000001',
    'Demo',
    'a0000000-0000-4000-8000-000000000001'
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_photos_sync_cursor ON photos;
DROP TRIGGER IF EXISTS trg_walls_sync_cursor ON walls;
DROP TRIGGER IF EXISTS trg_rooms_sync_cursor ON rooms;
DROP TRIGGER IF EXISTS trg_plans_sync_cursor ON plans;
DROP TRIGGER IF EXISTS trg_projects_sync_cursor ON projects;
DROP TRIGGER IF EXISTS trg_workspaces_sync_cursor ON workspaces;
DROP FUNCTION IF EXISTS bump_sync_cursor;

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

DROP SEQUENCE IF EXISTS sync_cursor_seq;

-- +goose StatementEnd
