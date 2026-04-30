package sync

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func roomSnapshot(r db.Room) (synctypes.EntitySnapshot, error) {
	pl := synctypes.RoomPayload{
		PlanID: r.PlanID,
		Name:   r.Name,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypeRoom,
		EntityID:   r.ID,
		Payload:    raw,
	}, nil
}

func (h *handler) pushRoom(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushRoomDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushRoomUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushRoomUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.RoomPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid room payload"}}
	}
	if payload.PlanID == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "planId is required"}}
	}

	planParentWS, err := workspaceOfPlan(ctx, q, payload.PlanID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "plan not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(planParentWS, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	row, err := q.GetRoomByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
		}
		out, err := q.UpsertRoom(ctx, db.UpsertRoomParams{
			ID:        op.EntityID,
			PlanID:    payload.PlanID,
			Name:      payload.Name,
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
	rws, err := workspaceOfRoom(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(rws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := roomSnapshot(row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertRoom(ctx, db.UpsertRoomParams{
		ID:        op.EntityID,
		PlanID:    payload.PlanID,
		Name:      payload.Name,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushRoomDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetRoomByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "room not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	rws, err := workspaceOfRoom(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(rws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := roomSnapshot(row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteRoom(ctx, db.SoftDeleteRoomParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}
