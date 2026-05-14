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

func TestPushPlanUpsert(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "plan-entity-1"
	const projectID = "proj-parent-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	planPayload := func(projectID, name string) json.RawMessage {
		b, err := json.Marshal(synctypes.PlanPayload{ProjectID: projectID, Name: name})
		require.NoError(t, err)
		return b
	}

	t.Run("invalid json payload — validation", func(t *testing.T) {
		q := &fakePlanPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         json.RawMessage(`not-json`),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "validation", res.PushError.Reason)
		require.Equal(t, "invalid plan payload", res.PushError.Message)
	})

	t.Run("empty projectId — validation", func(t *testing.T) {
		q := &fakePlanPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload("", "N"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "validation", res.PushError.Reason)
		require.Equal(t, "projectId and name are required", res.PushError.Message)
	})

	t.Run("empty name — validation", func(t *testing.T) {
		q := &fakePlanPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, ""),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "validation", res.PushError.Reason)
	})

	t.Run("parent project not found — notFound", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				require.Equal(t, projectID, id)
				return db.Project{}, pgx.ErrNoRows
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "P"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "notFound", res.PushError.Reason)
		require.Equal(t, "project not found", res.PushError.Message)
	})

	t.Run("parent project wrong workspace — forbidden", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{ID: projectID, WorkspaceID: "ws-other"}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "P"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("GetProjectByID error — internal", func(t *testing.T) {
		dbErr := errors.New("db down")
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{}, dbErr
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "P"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "internal", res.PushError.Reason)
	})

	t.Run("no plan row, OpCreate — applied", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{ID: projectID, WorkspaceID: wsID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				require.Equal(t, entityID, id)
				return db.Plan{}, pgx.ErrNoRows
			},
			upsertPlan: func(ctx context.Context, arg db.UpsertPlanParams) (db.Plan, error) {
				require.Equal(t, entityID, arg.ID)
				require.Equal(t, projectID, arg.ProjectID)
				require.Equal(t, "Floor 1", arg.Name)
				return db.Plan{SyncCursor: 77}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "Floor 1"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.True(t, res.Applied)
		require.Equal(t, int64(77), res.Cursor)
	})

	t.Run("no plan row, OpUpdate — notFound", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{}, pgx.ErrNoRows
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "X"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "notFound", res.PushError.Reason)
		require.Equal(t, "plan not found", res.PushError.Message)
	})

	t.Run("existing plan, workspace mismatch — forbidden", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				if id == projectID {
					return db.Project{ID: projectID, WorkspaceID: wsID}, nil
				}
				return db.Project{ID: id, WorkspaceID: "ws-other"}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{
					ID:        entityID,
					ProjectID: "proj-other",
					Name:      "S",
					UpdatedAt: validUpdated,
				}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "Mut"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.PushError)
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("same workspace, stale client — conflict", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{
					ID:        entityID,
					ProjectID: projectID,
					Name:      "Server",
					UpdatedAt: validUpdated,
				}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
			Payload:         planPayload(projectID, "Client"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.Conflict)
		require.Equal(t, "stale", res.Conflict.Reason)
		require.Equal(t, synctypes.EntityTypePlan, res.Conflict.ServerVersion.EntityType)
	})

	t.Run("same workspace, client wins — applied", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{
					ID:        entityID,
					ProjectID: projectID,
					UpdatedAt: validUpdated,
				}, nil
			},
			upsertPlan: func(ctx context.Context, arg db.UpsertPlanParams) (db.Plan, error) {
				return db.Plan{SyncCursor: 55}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityType:      synctypes.EntityTypePlan,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         planPayload(projectID, "Win"),
		}
		res := pushPlanUpsert(ctx, q, wsID, op)
		require.True(t, res.Applied)
		require.Equal(t, int64(55), res.Cursor)
	})
}

func TestPushPlanDelete(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "plan-del-1"
	const projectID = "proj-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	t.Run("not found", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{}, pgx.ErrNoRows
			},
		}
		res := pushPlanDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.NotNil(t, res.PushError)
		require.Equal(t, "notFound", res.PushError.Reason)
	})

	t.Run("forbidden", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ID: entityID, ProjectID: projectID, UpdatedAt: validUpdated}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		res := pushPlanDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.NotNil(t, res.PushError)
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("stale", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ID: entityID, ProjectID: projectID, UpdatedAt: validUpdated}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
		}
		res := pushPlanDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
		})
		require.NotNil(t, res.Conflict)
		require.Equal(t, "stale", res.Conflict.Reason)
	})

	t.Run("applied", func(t *testing.T) {
		q := &fakePlanPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ID: entityID, ProjectID: projectID, UpdatedAt: validUpdated}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			softDeletePlan: func(ctx context.Context, arg db.SoftDeletePlanParams) (db.Plan, error) {
				require.Equal(t, entityID, arg.ID)
				return db.Plan{SyncCursor: 99}, nil
			},
		}
		res := pushPlanDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(99), res.Cursor)
	})
}
