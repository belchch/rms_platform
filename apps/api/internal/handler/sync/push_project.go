package sync

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
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

func (h *handler) pushProject(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushProjectDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushProjectUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushProjectUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.ProjectPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid project payload"}}
	}
	if payload.Name == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "name is required"}}
	}

	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
		}
		out, err := q.UpsertProject(ctx, db.UpsertProjectParams{
			ID:          op.EntityID,
			WorkspaceID: wsID,
			Name:        payload.Name,
			Address:     payload.Address,
			Description: payload.Description,
			IsArchived:  payload.IsArchived,
			IsFavourite: payload.IsFavourite,
			UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(row.WorkspaceID, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := projectSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.UpsertProject(ctx, db.UpsertProjectParams{
		ID:          op.EntityID,
		WorkspaceID: wsID,
		Name:        payload.Name,
		Address:     payload.Address,
		Description: payload.Description,
		IsArchived:  payload.IsArchived,
		IsFavourite: payload.IsFavourite,
		UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushProjectDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
	}
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	if ve := validateWorkspace(row.WorkspaceID, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := projectSnapshot(row)
		if err != nil {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeleteProject(ctx, db.SoftDeleteProjectParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: err.Error()}}
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}
