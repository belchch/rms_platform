package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func TestPushProjectUpsert(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "proj-entity-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	namePayload := func(name string) json.RawMessage {
		b, err := json.Marshal(synctypes.ProjectPayload{Name: name})
		require.NoError(t, err)
		return b
	}

	t.Run("invalid json payload — validation", func(t *testing.T) {
		q := &fakeProjectPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         json.RawMessage(`not-json`),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "validation", res.pushError.Reason)
		require.Equal(t, "invalid project payload", res.pushError.Message)
	})

	t.Run("empty name — validation", func(t *testing.T) {
		q := &fakeProjectPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         namePayload(""),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "validation", res.pushError.Reason)
		require.Equal(t, "name is required", res.pushError.Message)
	})

	t.Run("no row, OpCreate — applied", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{}, pgx.ErrNoRows
			},
			upsertProject: func(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error) {
				require.Equal(t, entityID, arg.ID)
				require.Equal(t, wsID, arg.WorkspaceID)
				return db.Project{ID: entityID, SyncCursor: 9001}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         namePayload("New Project"),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.True(t, res.applied)
		require.Equal(t, int64(9001), res.cursor)
		require.Nil(t, res.pushError)
		require.Nil(t, res.conflict)
	})

	t.Run("no row, OpUpdate — notFound", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{}, pgx.ErrNoRows
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         namePayload("X"),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "notFound", res.pushError.Reason)
	})

	t.Run("GetProjectByID non-NoRows error — internal", func(t *testing.T) {
		dbErr := errors.New("connection refused")
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{}, dbErr
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         namePayload("P"),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "internal", res.pushError.Reason)
	})

	t.Run("wrong workspace — forbidden", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{
					ID:          entityID,
					WorkspaceID: "ws-other",
					Name:        "On Server",
					UpdatedAt:   validUpdated,
				}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         namePayload("Mutator"),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "forbidden", res.pushError.Reason)
	})

	t.Run("same workspace, client stale — conflict stale", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{
					ID:          entityID,
					WorkspaceID: wsID,
					Name:        "Server Title",
					UpdatedAt:   validUpdated,
				}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs,
			Payload:         namePayload("Client Title"),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.conflict)
		require.Equal(t, "stale", res.conflict.Reason)
		require.Equal(t, synctypes.EntityTypeProject, res.conflict.ServerVersion.EntityType)
	})

	t.Run("same workspace, client wins — applied", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{
					ID:          entityID,
					WorkspaceID: wsID,
					Name:        "Server Title",
					UpdatedAt:   validUpdated,
				}, nil
			},
			upsertProject: func(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error) {
				require.Equal(t, entityID, arg.ID)
				return db.Project{SyncCursor: 42}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypeProject,
			EntityID:        entityID,
			ClientUpdatedAt:   serverMs + 1,
			Payload:         namePayload("Winner"),
		}
		res := pushProjectUpsert(ctx, q, wsID, op)
		require.True(t, res.applied)
		require.Equal(t, int64(42), res.cursor)
		require.Nil(t, res.pushError)
	})
}

func TestPushProjectDelete(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "proj-del-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	t.Run("not found", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{}, pgx.ErrNoRows
			},
		}
		res := pushProjectDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.NotNil(t, res.pushError)
		require.Equal(t, "notFound", res.pushError.Reason)
	})

	t.Run("forbidden", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other", UpdatedAt: validUpdated}, nil
			},
		}
		res := pushProjectDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "forbidden", res.pushError.Reason)
	})

	t.Run("stale", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID, UpdatedAt: validUpdated}, nil
			},
		}
		res := pushProjectDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
		})
		require.Equal(t, "stale", res.conflict.Reason)
	})

	t.Run("applied", func(t *testing.T) {
		q := &fakeProjectPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID, UpdatedAt: validUpdated}, nil
			},
			softDeleteProject: func(ctx context.Context, arg db.SoftDeleteProjectParams) (db.Project, error) {
				require.Equal(t, entityID, arg.ID)
				return db.Project{SyncCursor: 31}, nil
			},
		}
		res := pushProjectDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.True(t, res.applied)
		require.Equal(t, int64(31), res.cursor)
	})
}
