package photo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func TestPushPhotoUpsert(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "photo-entity-1"
	const projectID = "proj-1"
	const planID = "plan-1"
	const roomID = "room-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	photoPayload := func(pl synctypes.PhotoPayload) json.RawMessage {
		b, err := json.Marshal(pl)
		require.NoError(t, err)
		return b
	}

	t.Run("invalid json — validation", func(t *testing.T) {
		q := &fakePhotoPushQuerier{}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload:         json.RawMessage(`not-json`),
		})
		require.Equal(t, "validation", res.PushError.Reason)
		require.Equal(t, "invalid photo payload", res.PushError.Message)
	})

	t.Run("contentType and parentId required", func(t *testing.T) {
		q := &fakePhotoPushQuerier{}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    "",
				ContentType: "",
			}),
		})
		require.Equal(t, "validation", res.PushError.Reason)
	})

	t.Run("parent not found", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{}, pgx.ErrNoRows
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/jpeg",
			}),
		})
		require.Equal(t, "notFound", res.PushError.Reason)
		require.Equal(t, "parent not found", res.PushError.Message)
	})

	t.Run("unsupported parentType — validation", func(t *testing.T) {
		q := &fakePhotoPushQuerier{}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityType("alien"),
				ParentID:    "x",
				ContentType: "image/jpeg",
			}),
		})
		require.Equal(t, "validation", res.PushError.Reason)
		require.True(t, strings.Contains(res.PushError.Message, "unsupported parentType"))
	})

	t.Run("parent workspace forbidden", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/jpeg",
			}),
		})
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("no photo, OpCreate — applied", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				require.Equal(t, entityID, id)
				return db.Photo{}, pgx.ErrNoRows
			},
			upsertPhotoableByOwner: func(ctx context.Context, arg db.UpsertPhotoableByOwnerParams) (db.Photoable, error) {
				require.Equal(t, "project", arg.OwnerType)
				require.Equal(t, projectID, arg.OwnerID)
				require.NotEmpty(t, arg.ID)
				return db.Photoable{ID: "pa-new"}, nil
			},
			upsertPhoto: func(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error) {
				require.Equal(t, entityID, arg.ID)
				require.Equal(t, "pa-new", arg.PhotoableID)
				require.Equal(t, "image/png", arg.ContentType)
				return db.Photo{SyncCursor: 500}, nil
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/png",
			}),
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(500), res.Cursor)
	})

	t.Run("no photo, OpUpdate — notFound", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{}, pgx.ErrNoRows
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/png",
			}),
		})
		require.Equal(t, "photo not found", res.PushError.Message)
	})

	t.Run("existing photo, client wins", func(t *testing.T) {
		photoRow := db.Photo{
			ID:          entityID,
			PhotoableID: "pa-1",
			ContentType: "image/jpeg",
			UpdatedAt:   validUpdated,
		}
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				require.Equal(t, entityID, id)
				return photoRow, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				require.Equal(t, "pa-1", id)
				return db.Photoable{ID: "pa-1", OwnerType: "project", OwnerID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			upsertPhoto: func(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error) {
				return db.Photo{SyncCursor: 88}, nil
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/png",
			}),
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(88), res.Cursor)
	})

	t.Run("parentMismatch conflict", func(t *testing.T) {
		photoRow := db.Photo{
			ID:          entityID,
			PhotoableID: "pa-1",
			UpdatedAt:   validUpdated,
		}
		q := &fakePhotoPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return photoRow, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				if id == "pa-1" {
					return db.Photoable{OwnerType: "project", OwnerID: projectID}, nil
				}
				return db.Photoable{}, errors.New("unexpected photoable id")
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeRoom,
				ParentID:    roomID,
				ContentType: "image/png",
			}),
		})
		require.NotNil(t, res.Conflict)
		require.Equal(t, "parentMismatch", res.Conflict.Reason)
	})

	t.Run("stale conflict", func(t *testing.T) {
		photoRow := db.Photo{
			ID:          entityID,
			PhotoableID: "pa-1",
			UpdatedAt:   validUpdated,
		}
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return photoRow, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				return db.Photoable{OwnerType: "project", OwnerID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/png",
			}),
		})
		require.Equal(t, "stale", res.Conflict.Reason)
	})

	t.Run("workspaceOfPhoto dataIntegrity", func(t *testing.T) {
		photoRow := db.Photo{
			ID:          entityID,
			PhotoableID: "pa-1",
			UpdatedAt:   validUpdated,
		}
		q := &fakePhotoPushQuerier{
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return photoRow, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				return db.Photoable{OwnerType: "unknown_kind", OwnerID: "x"}, nil
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpUpdate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeProject,
				ParentID:    projectID,
				ContentType: "image/png",
			}),
		})
		require.Equal(t, "dataIntegrity", res.PushError.Reason)
	})

	t.Run("parent via room chain", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
				return db.Room{PlanID: planID}, nil
			},
			getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
				return db.Plan{ProjectID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{}, pgx.ErrNoRows
			},
			upsertPhotoableByOwner: func(ctx context.Context, arg db.UpsertPhotoableByOwnerParams) (db.Photoable, error) {
				require.Equal(t, "room", arg.OwnerType)
				return db.Photoable{ID: "pa-r"}, nil
			},
			upsertPhoto: func(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error) {
				return db.Photo{SyncCursor: 9}, nil
			},
		}
		res := pushUpsert(ctx, q, wsID, synctypes.PushOperation{
			Op:              synctypes.OpCreate,
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
			Payload: photoPayload(synctypes.PhotoPayload{
				ParentType:  synctypes.EntityTypeRoom,
				ParentID:    roomID,
				ContentType: "image/jpeg",
			}),
		})
		require.True(t, res.Applied)
	})
}

func TestPushPhotoDelete(t *testing.T) {
	ctx := context.Background()
	const wsID = "ws-alpha"
	const entityID = "photo-del-1"
	const projectID = "proj-1"
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()
	validUpdated := pgtype.Timestamptz{Time: serverTime, Valid: true}

	t.Run("not found", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{}, pgx.ErrNoRows
			},
		}
		res := pushDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "notFound", res.PushError.Reason)
	})

	t.Run("forbidden", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{PhotoableID: "pa-1", UpdatedAt: validUpdated}, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				return db.Photoable{OwnerType: "project", OwnerID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: "ws-other"}, nil
			},
		}
		res := pushDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "forbidden", res.PushError.Reason)
	})

	t.Run("stale", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{PhotoableID: "pa-1", UpdatedAt: validUpdated}, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				return db.Photoable{OwnerType: "project", OwnerID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
		}
		res := pushDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs,
		})
		require.Equal(t, "stale", res.Conflict.Reason)
	})

	t.Run("applied", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{PhotoableID: "pa-1", UpdatedAt: validUpdated}, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				return db.Photoable{OwnerType: "project", OwnerID: projectID}, nil
			},
			getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
				return db.Project{WorkspaceID: wsID}, nil
			},
			softDeletePhoto: func(ctx context.Context, arg db.SoftDeletePhotoParams) (db.Photo, error) {
				return db.Photo{SyncCursor: 707}, nil
			},
		}
		res := pushDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.True(t, res.Applied)
		require.Equal(t, int64(707), res.Cursor)
	})

	t.Run("dataIntegrity", func(t *testing.T) {
		q := &fakePhotoPushQuerier{
			getPhotoByID: func(ctx context.Context, id string) (db.Photo, error) {
				return db.Photo{PhotoableID: "pa-1", UpdatedAt: validUpdated}, nil
			},
			getPhotoableByID: func(ctx context.Context, id string) (db.Photoable, error) {
				return db.Photoable{OwnerType: "bad", OwnerID: "x"}, nil
			},
		}
		res := pushDelete(ctx, q, wsID, synctypes.PushOperation{
			EntityID:        entityID,
			ClientUpdatedAt: serverMs + 1,
		})
		require.Equal(t, "dataIntegrity", res.PushError.Reason)
	})
}
