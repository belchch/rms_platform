package sync

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncdomain"
)

func applyPushOperation(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
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
		return syncdomain.PushStepResult{
			PushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported entityType"},
		}
	}
}
