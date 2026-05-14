package sync

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func workspaceOfPlan(ctx context.Context, q db.Querier, planID string) (string, error) {
	pl, err := q.GetPlanByID(ctx, planID)
	if err != nil {
		return "", err
	}
	p, err := q.GetProjectByID(ctx, pl.ProjectID)
	if err != nil {
		return "", err
	}
	return p.WorkspaceID, nil
}

func workspaceOfRoom(ctx context.Context, q db.Querier, roomID string) (string, error) {
	r, err := q.GetRoomByID(ctx, roomID)
	if err != nil {
		return "", err
	}
	return workspaceOfPlan(ctx, q, r.PlanID)
}

func workspaceOfWall(ctx context.Context, q db.Querier, wallID string) (string, error) {
	w, err := q.GetWallByID(ctx, wallID)
	if err != nil {
		return "", err
	}
	return workspaceOfRoom(ctx, q, w.RoomID)
}

type pushStepResult struct {
	applied   bool
	conflict  *synctypes.PushConflict
	pushError *synctypes.PushError
	cursor    int64
}

func internalPushErr(op synctypes.PushOperation, err error) pushStepResult {
	log.Error().Err(err).
		Str("clientOpId", op.ClientOpID).
		Str("entityId", op.EntityID).
		Msg("sync push internal error")
	return pushStepResult{pushError: &synctypes.PushError{Reason: "internal", Message: "internal server error"}}
}

func applyPushOperation(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.EntityType {
	case synctypes.EntityTypeProject:
		return pushProject(ctx, q, wsID, op)
	case synctypes.EntityTypePlan:
		return pushPlan(ctx, q, wsID, op)
	case synctypes.EntityTypeRoom:
		return pushRoom(ctx, q, wsID, op)
	case synctypes.EntityTypeWall:
		return pushWall(ctx, q, wsID, op)
	case synctypes.EntityTypePhoto:
		return pushPhoto(ctx, q, wsID, op)
	default:
		return pushStepResult{
			pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported entityType"},
		}
	}
}
