package sync

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/belchch/rms_platform/api/internal/db"
	mid "github.com/belchch/rms_platform/api/internal/middleware"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

type handler struct {
	pool *pgxpool.Pool
}

func pgTimeToEpochMs(t pgtype.Timestamptz) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.UnixMilli()
}

func epochMsToTimestamptz(ms int64) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.UnixMilli(ms), Valid: true}
}

func lwwWins(clientMs int64, serverUpdated pgtype.Timestamptz) bool {
	return clientMs > pgTimeToEpochMs(serverUpdated)
}

func workspaceOfPlan(ctx context.Context, q *db.Queries, planID string) (string, error) {
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

func workspaceOfRoom(ctx context.Context, q *db.Queries, roomID string) (string, error) {
	r, err := q.GetRoomByID(ctx, roomID)
	if err != nil {
		return "", err
	}
	return workspaceOfPlan(ctx, q, r.PlanID)
}

func workspaceOfWall(ctx context.Context, q *db.Queries, wallID string) (string, error) {
	w, err := q.GetWallByID(ctx, wallID)
	if err != nil {
		return "", err
	}
	return workspaceOfRoom(ctx, q, w.RoomID)
}

func validateWorkspace(actualWS, jwtWS string) *synctypes.PushError {
	if actualWS != jwtWS {
		return &synctypes.PushError{Reason: "forbidden", Message: "entity belongs to another workspace"}
	}
	return nil
}

type pushStepResult struct {
	applied   bool
	conflict  *synctypes.PushConflict
	pushError *synctypes.PushError
	cursor    int64
}

func (h *handler) push(ctx context.Context, in *PushInput) (*PushOutput, error) {
	wsID, ok := mid.WorkspaceID(ctx)
	if !ok {
		return nil, huma.NewError(http.StatusUnauthorized, "Unauthorized")
	}

	out := &PushOutput{}
	out.Body.Applied = []string{}
	out.Body.Conflicts = []synctypes.PushConflict{}
	out.Body.Errors = []synctypes.PushError{}
	out.Body.Cursor = 0

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync push begin: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)
	var maxCursor int64

	for i, op := range in.Body.Operations {
		sp := "sp_" + strconv.Itoa(i)
		if _, err := tx.Exec(ctx, "SAVEPOINT "+sp); err != nil {
			return nil, fmt.Errorf("sync push savepoint: %w", err)
		}

		res := h.applyPushOperation(ctx, qtx, wsID, op)

		if res.pushError != nil {
			res.pushError.ClientOpID = op.ClientOpID
			out.Body.Errors = append(out.Body.Errors, *res.pushError)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.conflict != nil {
			res.conflict.ClientOpID = op.ClientOpID
			out.Body.Conflicts = append(out.Body.Conflicts, *res.conflict)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.applied {
			out.Body.Applied = append(out.Body.Applied, op.ClientOpID)
			if res.cursor > maxCursor {
				maxCursor = res.cursor
			}
		}
		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
			return nil, fmt.Errorf("sync push release savepoint: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("sync push commit: %w", err)
	}

	out.Body.Cursor = maxCursor
	return out, nil
}

func (h *handler) applyPushOperation(ctx context.Context, q *db.Queries, wsID string, op synctypes.PushOperation) pushStepResult {
	switch op.EntityType {
	case synctypes.EntityTypeProject:
		return h.pushProject(ctx, q, wsID, op)
	case synctypes.EntityTypePlan:
		return h.pushPlan(ctx, q, wsID, op)
	case synctypes.EntityTypeRoom:
		return h.pushRoom(ctx, q, wsID, op)
	case synctypes.EntityTypeWall:
		return h.pushWall(ctx, q, wsID, op)
	case synctypes.EntityTypePhoto:
		return h.pushPhoto(ctx, q, wsID, op)
	default:
		return pushStepResult{
			pushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported entityType"},
		}
	}
}
