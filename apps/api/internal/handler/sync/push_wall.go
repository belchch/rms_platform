package sync

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func wallSnapshot(w db.Wall) (synctypes.EntitySnapshot, error) {
	pl := synctypes.WallPayload{RoomID: w.RoomID}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypeWall,
		EntityID:   w.ID,
		Payload:    raw,
	}, nil
}

func (h *handler) pushWall(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushWallDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushWallUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushWallUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.WallPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid wall payload"}}
	}
	if payload.RoomID == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "roomId is required"}}
	}

	rws, err := workspaceOfRoom(ctx, q, payload.RoomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(rws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	row, err := q.GetWallByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "wall not found"}}
		}
		out, err := q.UpsertWall(ctx, db.UpsertWallParams{
			ID:        op.EntityID,
			RoomID:    payload.RoomID,
			UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	wws, err := workspaceOfWall(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(wws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	wins, err := lwwWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return internalPushErr(op, err)
	}
	if !wins {
		snap, err := wallSnapshot(row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertWall(ctx, db.UpsertWallParams{
		ID:        op.EntityID,
		RoomID:    payload.RoomID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushWallDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetWallByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "wall not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	wws, err := workspaceOfWall(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(wws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	wins, err := lwwWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return internalPushErr(op, err)
	}
	if !wins {
		snap, err := wallSnapshot(row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteWall(ctx, db.SoftDeleteWallParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}
