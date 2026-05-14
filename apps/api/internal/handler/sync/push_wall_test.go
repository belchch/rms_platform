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

func TestPushWallUpsert(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "wall-entity-1"
	const roomID = "room-parent-1"
	const planID = "plan-1"
	const projectID = "proj-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	wallPayload := func(roomID string) json.RawMessage {
		b, err := json.Marshal(synctypes.WallPayload{RoomID: roomID})
		require.NoError(t, err)
		return b
	}

	chainOK := func() *fakeWallPushQuerier {
		return &fakeWallPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{ID: roomID, PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
		}
	}

	t.Run("invalid json — validation", func(t *testing.T) {
		q := &fakeWallPushQuerier{}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         json.RawMessage(`not-json`),
		})
		require.Equal(t, "validation", res.PushError.Reason)
	})

	t.Run("empty roomId — validation", func(t *testing.T) {
		q := &fakeWallPushQuerier{}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(""),
		})
		require.Equal(t, "roomId is required", res.PushError.Message)
	})

	t.Run("room not found — notFound", func(t *testing.T) {
		q := &fakeWallPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{}, pgx.ErrNoRows
			},
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.Equal(t, "notFound", res.PushError.Reason)
		require.Equal(t, "room not found", res.PushError.Message)
	})

	t.Run("parent room workspace forbidden", func(t *testing.T) {
		q := &fakeWallPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("no wall, OpCreate — applied", func(t *testing.T) {
		q := chainOK()
		q.getWallByID = func(ctx context.Context, id string) (db.Wall, error) {
			require.Equal(t, entityID, id)
			return db.Wall{}, pgx.ErrNoRows
		}
		q.upsertWall = func(ctx context.Context, arg db.UpsertWallParams) (db.Wall, error) {
			require.Equal(t, entityID, arg.ID)
			require.Equal(t, roomID, arg.RoomID)
			return db.Wall{SyncCursor: 8}, nil
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(8), res.Cursor)
	})

	t.Run("no wall, OpUpdate — notFound", func(t *testing.T) {
		q := chainOK()
		q.getWallByID = func(ctx context.Context, id string) (db.Wall, error) {
			return db.Wall{}, pgx.ErrNoRows
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.Equal(t, "wall not found", res.PushError.Message)
	})

	t.Run("existing wall wrong workspace", func(t *testing.T) {
		q := chainOK()
		q.getWallByID = func(ctx context.Context, id string) (db.Wall, error) {
			return db.Wall{ID: entityID, RoomID: "room-other", UpdatedAt: validUpdated}, nil
		}
		q.getRoomByID = func(ctx context.Context, id string) (db.Room, error) {
			if id == "room-other" {
				return db.Room{PlanID: "plan-other"}, nil
			}
			return db.Room{PlanID: planID}, nil
		}
		q.getPlanByID = func(ctx context.Context, id string) (db.Plan, error) {
			if id == "plan-other" {
				return db.Plan{ProjectID: "proj-other"}, nil
			}
			return db.Plan{ProjectID: projectID}, nil
		}
		q.getProjectByID = func(ctx context.Context, id string) (db.Project, error) {
			if id == "proj-other" {
				return db.Project{WorkspaceID: "ws-other"}, nil
			}
			return db.Project{WorkspaceID: wsID}, nil
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("stale", func(t *testing.T) {
		q := chainOK()
		q.getWallByID = func(ctx context.Context, id string) (db.Wall, error) {
			return db.Wall{RoomID: roomID, UpdatedAt: validUpdated}, nil
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
			Payload:         wallPayload(roomID),
		})
		require.Equal(t, "stale", res.Conflict.Reason)
	})

	t.Run("client wins", func(t *testing.T) {
		q := chainOK()
		q.getWallByID = func(ctx context.Context, id string) (db.Wall, error) {
			return db.Wall{RoomID: roomID, UpdatedAt: validUpdated}, nil
		}
		q.upsertWall = func(ctx context.Context, arg db.UpsertWallParams) (db.Wall, error) {
			return db.Wall{SyncCursor: 6}, nil
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(6), res.Cursor)
	})

	t.Run("GetRoomByID error — internal", func(t *testing.T) {
		dbErr := errors.New("db")
		q := &fakeWallPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{}, dbErr
			},
		}
		res := pushWallUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         wallPayload(roomID),
		})
		require.Equal(t, "internal", res.PushError.Reason)
	})
}

func TestPushWallDelete(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "wall-del-1"
	const roomID = "room-1"
	const planID = "plan-1"
	const projectID = "proj-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	t.Run("not found", func(t *testing.T) {
		q := &fakeWallPushQuerier{
			getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
				return db.Wall{}, pgx.ErrNoRows
			},
		}
		res := pushWallDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "notFound", res.PushError.Reason)
	})

	t.Run("forbidden", func(t *testing.T) {
		q := &fakeWallPushQuerier{
			getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
				return db.Wall{RoomID: roomID, UpdatedAt: validUpdated}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		res := pushWallDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("stale", func(t *testing.T) {
		q := &fakeWallPushQuerier{
			getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
				return db.Wall{RoomID: roomID, UpdatedAt: validUpdated}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
		}
		res := pushWallDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
		})
		require.Equal(t, "stale", res.Conflict.Reason)
	})

	t.Run("applied", func(t *testing.T) {
		q := &fakeWallPushQuerier{
			getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
				return db.Wall{RoomID: roomID, UpdatedAt: validUpdated}, nil
			},
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			softDeleteWall: func(ctx context.Context, arg db.SoftDeleteWallParams) (db.Wall, error) {
				return db.Wall{SyncCursor: 22}, nil
			},
		}
		res := pushWallDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(22), res.Cursor)
	})
}
