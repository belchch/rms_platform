package sync

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func planSnapshot(p db.Plan) (synctypes.EntitySnapshot, error) {
	pl := synctypes.PlanPayload{
		ProjectID:   p.ProjectID,
		Name:        p.Name,
		PayloadJSON: p.PayloadJson,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return synctypes.EntitySnapshot{}, err
	}
	return synctypes.EntitySnapshot{
		EntityType: synctypes.EntityTypePlan,
		EntityID:   p.ID,
		Payload:    raw,
	}, nil
}

func (h *handler) pushPlan(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.Op {
	case synctypes.OpDelete:
		return h.pushPlanDelete(ctx, q, wsID, op)
	case synctypes.OpCreate, synctypes.OpUpdate:
		return h.pushPlanUpsert(ctx, q, wsID, op)
	default:
		return pushStepResult{pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported op"}}
	}
}

func (h *handler) pushPlanUpsert(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	var payload synctypes.PlanPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "invalid plan payload"}}
	}
	if payload.ProjectID == "" || payload.Name == "" {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "validation", Message: "projectId and name are required"}}
	}

	parentProj, err := q.GetProjectByID(ctx, payload.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(parentProj.WorkspaceID, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	row, err := q.GetPlanByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "plan not found"}}
		}
		out, err := q.UpsertPlan(ctx, db.UpsertPlanParams{
			ID:          op.EntityID,
			ProjectID:   payload.ProjectID,
			Name:        payload.Name,
			PayloadJson: payload.PayloadJSON,
			UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
		})
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{applied: true, cursor: out.SyncCursor}
	}
	if err != nil {
		return internalPushErr(op, err)
	}

	pws, err := workspaceOfPlan(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(pws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}

	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := planSnapshot(row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}

	out, err := q.UpsertPlan(ctx, db.UpsertPlanParams{
		ID:          op.EntityID,
		ProjectID:   payload.ProjectID,
		Name:        payload.Name,
		PayloadJson: payload.PayloadJSON,
		UpdatedAt:   epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}

func (h *handler) pushPlanDelete(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	row, err := q.GetPlanByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushStepResult{pushError: &synctypes.PushError{Reason: "notFound", Message: "plan not found"}}
	}
	if err != nil {
		return internalPushErr(op, err)
	}
	pws, err := workspaceOfPlan(ctx, q, row.ID)
	if err != nil {
		return internalPushErr(op, err)
	}
	if ve := validateWorkspace(pws, wsID); ve != nil {
		return pushStepResult{pushError: ve}
	}
	if !lwwWins(op.ClientUpdatedAt, row.UpdatedAt) {
		snap, err := planSnapshot(row)
		if err != nil {
			return internalPushErr(op, err)
		}
		return pushStepResult{conflict: &synctypes.PushConflict{Reason: "stale", ServerVersion: snap}}
	}
	out, err := q.SoftDeletePlan(ctx, db.SoftDeletePlanParams{
		ID:        op.EntityID,
		UpdatedAt: epochMsToTimestamptz(op.ClientUpdatedAt),
	})
	if err != nil {
		return internalPushErr(op, err)
	}
	return pushStepResult{applied: true, cursor: out.SyncCursor}
}
