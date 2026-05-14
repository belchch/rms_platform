package wall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncdomain"
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

func ApplyPush(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return pushDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return pushUpsert(ctx, q, wsID, op)
	default:
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func pushUpsert(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	var payload synctypes.WallPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "invalid wall payload"}}
	}
	if payload.RoomID == "" {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "roomId is required"}}
	}

	rws, err := syncdomain.WorkspaceOfRoom(ctx, q, payload.RoomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(rws, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}

	row, err := q.GetWallByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "wall not found"}}
		}
		out, err := q.UpsertWall(ctx, db.UpsertWallParams{
			ID:        op.EntityID,
			RoomID:    payload.RoomID,
			UpdatedAt: syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	wws, err := syncdomain.WorkspaceOfWall(ctx, q, row.ID)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(wws, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}
	wins, err := syncdomain.LWWWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if !wins {
		snap, err := wallSnapshot(row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertWall(ctx, db.UpsertWallParams{
		ID:        op.EntityID,
		RoomID:    payload.RoomID,
		UpdatedAt: syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}

func pushDelete(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	row, err := q.GetWallByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "wall not found"}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	wws, err := syncdomain.WorkspaceOfWall(ctx, q, row.ID)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(wws, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}
	wins, err := syncdomain.LWWWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if !wins {
		snap, err := wallSnapshot(row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteWall(ctx, db.SoftDeleteWallParams{
		ID:        op.EntityID,
		UpdatedAt: syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}

func AppendPullChanges(changes []synctypes.PullChange, rows []db.Wall) ([]synctypes.PullChange, error) {
	return syncdomain.AppendPullChangesFromRows(changes, rows, func(w db.Wall) (synctypes.PullChange, error) {
		snap, err := wallSnapshot(w)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeWall)).Str("entityId", w.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull wall snapshot: %w", err)
		}
		pc, err := syncdomain.PullChangeFromSnapshot(snap, w.UpdatedAt, w.SyncCursor, w.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeWall)).Str("entityId", w.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull wall snapshot: %w", err)
		}
		return pc, nil
	})
}
