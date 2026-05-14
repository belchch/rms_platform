package syncdomain

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

type PushStepResult struct {
	Applied   bool
	Conflict  *synctypes.PushConflict
	PushError *synctypes.PushError
	Cursor    int64
}

func InternalPushErr(op synctypes.PushOperation, err error) PushStepResult {
	log.Error().Err(err).
		Str("clientOpId", op.ClientOpID).
		Str("entityId", op.EntityID).
		Msg("sync push internal error")
	return PushStepResult{PushError: &synctypes.PushError{Reason: "internal", Message: "internal server error"}}
}

func LWWWins(clientMs int64, serverUpdated pgtype.Timestamptz) (bool, error) {
	if !serverUpdated.Valid {
		return false, fmt.Errorf("server updated_at is invalid: NOT NULL invariant violated")
	}
	return clientMs > serverUpdated.Time.UnixMilli(), nil
}

func ValidateWorkspace(actualWS, jwtWS string) *synctypes.PushError {
	if actualWS != jwtWS {
		return &synctypes.PushError{Reason: "forbidden", Message: "entity belongs to another workspace"}
	}
	return nil
}

func WorkspaceOfPlan(ctx context.Context, q db.Querier, planID string) (string, error) {
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

func WorkspaceOfRoom(ctx context.Context, q db.Querier, roomID string) (string, error) {
	r, err := q.GetRoomByID(ctx, roomID)
	if err != nil {
		return "", err
	}
	return WorkspaceOfPlan(ctx, q, r.PlanID)
}

func WorkspaceOfWall(ctx context.Context, q db.Querier, wallID string) (string, error) {
	w, err := q.GetWallByID(ctx, wallID)
	if err != nil {
		return "", err
	}
	return WorkspaceOfRoom(ctx, q, w.RoomID)
}
