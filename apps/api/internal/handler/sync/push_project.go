package sync

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncdomain"
)

func projectSnapshot(p db.Project) (synctypes.EntitySnapshot, error) {
	pl := synctypes.ProjectPayload{
		Name:        p.Name,
		Address:     p.Address,
		Description: p.Description,
		IsArchived:  p.IsArchived,
		IsFavourite: p.IsFavourite,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypeProject,
		EntityID:   p.ID,
		Payload:    raw,
	}, nil
}

func pushProject(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return pushProjectDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return pushProjectUpsert(ctx, q, wsID, op)
	default:
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func pushProjectUpsert(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	var payload synctypes.ProjectPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "invalid project payload"}}
	}
	if payload.Name == "" {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "name is required"}}
	}

	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
		}
		out, err := q.UpsertProject(ctx, db.UpsertProjectParams{
			ID:          op.EntityID,
			WorkspaceID: wsID,
			Name:        payload.Name,
			Address:     payload.Address,
			Description: payload.Description,
			IsArchived:  payload.IsArchived,
			IsFavourite: payload.IsFavourite,
			UpdatedAt:   syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(row.WorkspaceID, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}
	wins, err := syncdomain.LWWWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if !wins {
		snap, err := projectSnapshot(row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertProject(ctx, db.UpsertProjectParams{
		ID:          op.EntityID,
		WorkspaceID: wsID,
		Name:        payload.Name,
		Address:     payload.Address,
		Description: payload.Description,
		IsArchived:  payload.IsArchived,
		IsFavourite: payload.IsFavourite,
		UpdatedAt:   syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}

func pushProjectDelete(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
	}
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if ve := syncdomain.ValidateWorkspace(row.WorkspaceID, wsID); ve != nil {
		return syncdomain.PushStepResult{PushError: ve}
	}
	wins, err := syncdomain.LWWWins(op.ClientUpdatedAt, row.UpdatedAt)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	if !wins {
		snap, err := projectSnapshot(row)
		if err != nil {
			return syncdomain.InternalPushErr(op, err)
		}
		return syncdomain.PushStepResult{Conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteProject(ctx, db.SoftDeleteProjectParams{
		ID:        op.EntityID,
		UpdatedAt: syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}
