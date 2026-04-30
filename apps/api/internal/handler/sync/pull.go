package sync

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	mid "github.com/belchch/rms_platform/api/internal/middleware"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func (h *handler) pull(ctx context.Context, in *PullInput) (*PullOutput, error) {
	wsID, ok := mid.WorkspaceID(ctx)
	if !ok {
		return nil, huma.NewError(http.StatusUnauthorized, "Unauthorized")
	}

	if in.Since < 0 {
		return nil, huma.NewError(http.StatusBadRequest, "since must be >= 0")
	}

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
		IsoLevel:   pgx.RepeatableRead,
	})
	if err != nil {
		return nil, fmt.Errorf("sync pull begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	since := in.Since

	projects, err := q.ListProjectsSince(ctx, db.ListProjectsSinceParams{
		WorkspaceID: wsID,
		SyncCursor:  since,
	})
	if err != nil {
		return nil, fmt.Errorf("sync pull projects: %w", err)
	}
	plans, err := q.ListPlansSince(ctx, db.ListPlansSinceParams{
		WorkspaceID: wsID,
		SyncCursor:  since,
	})
	if err != nil {
		return nil, fmt.Errorf("sync pull plans: %w", err)
	}
	rooms, err := q.ListRoomsSince(ctx, db.ListRoomsSinceParams{
		WorkspaceID: wsID,
		SyncCursor:  since,
	})
	if err != nil {
		return nil, fmt.Errorf("sync pull rooms: %w", err)
	}
	walls, err := q.ListWallsSince(ctx, db.ListWallsSinceParams{
		WorkspaceID: wsID,
		SyncCursor:  since,
	})
	if err != nil {
		return nil, fmt.Errorf("sync pull walls: %w", err)
	}
	photos, err := q.ListPhotosSince(ctx, db.ListPhotosSinceParams{
		WorkspaceID: wsID,
		SyncCursor:  since,
	})
	if err != nil {
		return nil, fmt.Errorf("sync pull photos: %w", err)
	}

	var changes []synctypes.PullChange
	if changes, err = pullAppendProjects(changes, projects); err != nil {
		return nil, err
	}
	if changes, err = pullAppendPlans(changes, plans); err != nil {
		return nil, err
	}
	if changes, err = pullAppendRooms(changes, rooms); err != nil {
		return nil, err
	}
	if changes, err = pullAppendWalls(changes, walls); err != nil {
		return nil, err
	}
	if changes, err = pullAppendPhotos(changes, photos); err != nil {
		return nil, err
	}

	sort.Slice(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if a.SyncCursor != b.SyncCursor {
			return a.SyncCursor < b.SyncCursor
		}
		if a.EntityType != b.EntityType {
			return a.EntityType < b.EntityType
		}
		return a.EntityID < b.EntityID
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("sync pull commit: %w", err)
	}

	out := &PullOutput{}
	out.Body.Changes = changes
	cursor := since
	for _, c := range changes {
		if c.SyncCursor > cursor {
			cursor = c.SyncCursor
		}
	}
	out.Body.Cursor = cursor
	return out, nil
}
