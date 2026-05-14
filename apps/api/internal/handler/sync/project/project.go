package project

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
	var payload synctypes.ProjectPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "invalid project payload"}}
	}
	if payload.Name == "" {
		return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "validation", Message: "name is required"}}
	}

	params := db.UpsertProjectParams{
		ID:          op.EntityID,
		WorkspaceID: wsID,
		Name:        payload.Name,
		Address:     payload.Address,
		Description: payload.Description,
		IsArchived:  payload.IsArchived,
		IsFavourite: payload.IsFavourite,
		UpdatedAt:   syncdomain.EpochMsToTimestamptz(op.ClientUpdatedAt),
	}

	row, err := q.GetProjectByID(ctx, op.EntityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if op.Op == synctypes.OpUpdate {
			return syncdomain.PushStepResult{PushError: &synctypes.PushError{Reason: "notFound", Message: "project not found"}}
		}
		out, err := q.UpsertProject(ctx, params)
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
	out, err := q.UpsertProject(ctx, params)
	if err != nil {
		return syncdomain.InternalPushErr(op, err)
	}
	return syncdomain.PushStepResult{Applied: true, Cursor: out.SyncCursor}
}

func pushDelete(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
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

func AppendPullChanges(changes []synctypes.PullChange, rows []db.Project) ([]synctypes.PullChange, error) {
	return syncdomain.AppendPullChangesFromRows(changes, rows, func(p db.Project) (synctypes.PullChange, error) {
		snap, err := projectSnapshot(p)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeProject)).Str("entityId", p.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull project snapshot: %w", err)
		}
		pc, err := syncdomain.PullChangeFromSnapshot(snap, p.UpdatedAt, p.SyncCursor, p.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeProject)).Str("entityId", p.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull project snapshot: %w", err)
		}
		return pc, nil
	})
}
