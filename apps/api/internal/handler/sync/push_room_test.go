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

func TestPushRoomUpsert(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "room-entity-1"
	const planID = "plan-parent-1"
	const projectID = "proj-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}
	name := "R1"

	roomPayload := func(planID string, name *string) json.RawMessage {
		b, err := json.Marshal(synctypes.RoomPayload{PlanID: planID, Name: name})
		require.NoError(t, err)
		return b
	}

	h := &handler{}

	t.Run("invalid json — validation", func(t *testing.T) {
		q := &fakeRoomPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         json.RawMessage(`not-json`),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "validation", res.pushError.Reason)
	})

	t.Run("empty planId — validation", func(t *testing.T) {
		q := &fakeRoomPushQuerier{}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload("", &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "planId is required", res.pushError.Message)
	})

	t.Run("plan not found — notFound", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{}, pgx.ErrNoRows
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "notFound", res.pushError.Reason)
		require.Equal(t, "plan not found", res.pushError.Message)
	})

	t.Run("parent plan workspace mismatch — forbidden", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ID: planID, ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "forbidden", res.pushError.Reason)
	})

	t.Run("no room, OpCreate — applied", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ID: planID, ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				require.Equal(t, entityID, id)
				return db.Room{}, pgx.ErrNoRows
			},
			upsertRoom: func(ctx context.Context, arg db.UpsertRoomParams) (db.Room, error) {
				require.Equal(t, entityID, arg.ID)
				require.Equal(t, planID, arg.PlanID)
				return db.Room{SyncCursor: 12}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.True(t, res.applied)
		require.Equal(t, int64(12), res.cursor)
	})

	t.Run("no room, OpUpdate — notFound", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{}, pgx.ErrNoRows
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "room not found", res.pushError.Message)
	})

	t.Run("existing room forbidden workspace", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				if id == "plan-other" {
					return db.Plan{ProjectID: "proj-other"}, nil
				}
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				if id == projectID {
					return db.Project{WorkspaceID: wsID}, nil
				}
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{ID: entityID, PlanID: "plan-other", UpdatedAt: validUpdated}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.pushError)
		require.Equal(t, "forbidden", res.pushError.Reason)
	})

	t.Run("stale conflict", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID, UpdatedAt: validUpdated}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.NotNil(t, res.conflict)
		require.Equal(t, "stale", res.conflict.Reason)
	})

	t.Run("client wins", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID, UpdatedAt: validUpdated}, nil
			},
			upsertRoom: func(ctx context.Context, arg db.UpsertRoomParams) (db.Room, error) {
				return db.Room{SyncCursor: 3}, nil
			},
		}
		op := synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		}
		res := h.pushRoomUpsert(ctx, q, wsID, op)
		require.True(t, res.applied)
		require.Equal(t, int64(3), res.cursor)
	})

	t.Run("GetPlanByID error — internal", func(t *testing.T) {
		dbErr := errors.New("timeout")
		q := &fakeRoomPushQuerier{
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{}, dbErr
			},
		}
		res := h.pushRoomUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         roomPayload(planID, &name),
		})
		require.NotNil(t, res.pushError)
		require.Equal(t, "internal", res.pushError.Reason)
	})
}

func TestPushRoomDelete(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "room-del-1"
	const planID = "plan-1"
	const projectID = "proj-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}
	h := &handler{}

	t.Run("not found", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{}, pgx.ErrNoRows
			},
		}
		res := h.pushRoomDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "notFound", res.pushError.Reason)
	})

	t.Run("forbidden", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{ID: entityID, PlanID: planID, UpdatedAt: validUpdated}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		res := h.pushRoomDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "forbidden", res.pushError.Reason)
	})

	t.Run("stale", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID, UpdatedAt: validUpdated}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
		}
		res := h.pushRoomDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
		})
		require.Equal(t, "stale", res.conflict.Reason)
	})

	t.Run("applied", func(t *testing.T) {
		q := &fakeRoomPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID, UpdatedAt: validUpdated}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			softDeleteRoom: func(ctx context.Context, arg db.SoftDeleteRoomParams) (db.Room, error) {
				return db.Room{SyncCursor: 44}, nil
			},
		}
		res := h.pushRoomDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.True(t, res.applied)
		require.Equal(t, int64(44), res.cursor)
	})
}
