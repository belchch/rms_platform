package sync

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/belchch/rms_platform/api/internal/db"
	mid "github.com/belchch/rms_platform/api/internal/middleware"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func timestamptzEpochMs(t pgtype.Timestamptz) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.UnixMilli()
}

func timestamptzEpochMsPtr(t pgtype.Timestamptz) *int64 {
	if !t.Valid {
		return nil
	}
	ms := t.Time.UnixMilli()
	return &ms
}

func pullChangeFromSnapshot(snap synctypes.EntitySnapshot, updatedAt pgtype.Timestamptz, syncCursor int64, deletedAt pgtype.Timestamptz) synctypes.PullChange {
	return synctypes.PullChange{
		EntityType: snap.EntityType,
		EntityID:   snap.EntityID,
		Payload:    snap.Payload,
		UpdatedAt:  timestamptzEpochMs(updatedAt),
		SyncCursor: syncCursor,
		DeletedAt:  timestamptzEpochMsPtr(deletedAt),
	}
}

func (h *handler) pull(ctx context.Context, in *PullInput) (*PullOutput, error) {
	wsID, ok := mid.WorkspaceID(ctx)
	if !ok {
		return nil, huma.NewError(http.StatusUnauthorized, "Unauthorized")
	}

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("sync pull begin: %w", err)
	}
	defer tx.Rollback(ctx)

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

	for _, p := range projects {
		snap, err := projectSnapshot(p)
		if err != nil {
			return nil, fmt.Errorf("sync pull project snapshot: %w", err)
		}
		changes = append(changes, pullChangeFromSnapshot(snap, p.UpdatedAt, p.SyncCursor, p.DeletedAt))
	}
	for _, p := range plans {
		snap, err := planSnapshot(p)
		if err != nil {
			return nil, fmt.Errorf("sync pull plan snapshot: %w", err)
		}
		changes = append(changes, pullChangeFromSnapshot(snap, p.UpdatedAt, p.SyncCursor, p.DeletedAt))
	}
	for _, r := range rooms {
		snap, err := roomSnapshot(r)
		if err != nil {
			return nil, fmt.Errorf("sync pull room snapshot: %w", err)
		}
		changes = append(changes, pullChangeFromSnapshot(snap, r.UpdatedAt, r.SyncCursor, r.DeletedAt))
	}
	for _, w := range walls {
		snap, err := wallSnapshot(w)
		if err != nil {
			return nil, fmt.Errorf("sync pull wall snapshot: %w", err)
		}
		changes = append(changes, pullChangeFromSnapshot(snap, w.UpdatedAt, w.SyncCursor, w.DeletedAt))
	}
	for _, p := range photos {
		snap, err := photoSnapshot(ctx, q, p)
		if err != nil {
			return nil, fmt.Errorf("sync pull photo snapshot: %w", err)
		}
		changes = append(changes, pullChangeFromSnapshot(snap, p.UpdatedAt, p.SyncCursor, p.DeletedAt))
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].SyncCursor < changes[j].SyncCursor
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
