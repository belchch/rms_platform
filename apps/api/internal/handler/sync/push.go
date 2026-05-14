package sync

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/photo"
	"github.com/belchch/rms_platform/api/internal/handler/sync/plan"
	"github.com/belchch/rms_platform/api/internal/handler/sync/project"
	"github.com/belchch/rms_platform/api/internal/handler/sync/room"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncdomain"
	"github.com/belchch/rms_platform/api/internal/handler/sync/wall"
	mid "github.com/belchch/rms_platform/api/internal/middleware"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

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

	// DB infrastructure failures (Begin, Commit, savepoint ops) are the only
	// cases where POST /sync/push returns a non-200 status. All entity-level
	// conflicts and validation errors are reported in the 200 body (errors[]/conflicts[]).
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Str("workspaceId", wsID).Msg("sync push begin failed")
		return nil, fmt.Errorf("sync push begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)
	var maxCursor int64

	for i, op := range in.Body.Operations {
		sp := "sp_" + strconv.Itoa(i)
		if _, err := tx.Exec(ctx, "SAVEPOINT "+sp); err != nil {
			log.Error().Err(err).Str("workspaceId", wsID).Int("opIndex", i).Msg("sync push savepoint failed")
			return nil, fmt.Errorf("sync push savepoint: %w", err)
		}

		res := applyPushOperation(ctx, qtx, wsID, op)

		if res.PushError != nil {
			res.PushError.ClientOpID = op.ClientOpID
			out.Body.Errors = append(out.Body.Errors, *res.PushError)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				log.Error().Err(rbErr).Str("workspaceId", wsID).Int("opIndex", i).Str("after", "pushError").Msg("sync push rollback savepoint failed")
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.Conflict != nil {
			res.Conflict.ClientOpID = op.ClientOpID
			out.Body.Conflicts = append(out.Body.Conflicts, *res.Conflict)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				log.Error().Err(rbErr).Str("workspaceId", wsID).Int("opIndex", i).Str("after", "conflict").Msg("sync push rollback savepoint failed")
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.Applied {
			out.Body.Applied = append(out.Body.Applied, op.ClientOpID)
			if res.Cursor > maxCursor {
				maxCursor = res.Cursor
			}
		}
		if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
			log.Error().Err(err).Str("workspaceId", wsID).Int("opIndex", i).Msg("sync push release savepoint failed")
			return nil, fmt.Errorf("sync push release savepoint: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Str("workspaceId", wsID).Msg("sync push commit failed")
		return nil, fmt.Errorf("sync push commit: %w", err)
	}

	out.Body.Cursor = maxCursor

	evt := log.Debug()
	if len(out.Body.Conflicts) > 0 || len(out.Body.Errors) > 0 {
		evt = log.Warn()
	}
	evt.Int("operations", len(in.Body.Operations)).
		Int("applied", len(out.Body.Applied)).
		Int("conflicts", len(out.Body.Conflicts)).
		Int("errors", len(out.Body.Errors)).
		Int64("cursor", maxCursor).
		Msg("sync push completed")

	return out, nil
}

func applyPushOperation(ctx context.Context, q db.Querier, wsID string, op synctypes.PushOperation) syncdomain.PushStepResult {
	switch op.EntityType {
	case synctypes.EntityTypeProject:
		return project.ApplyPush(ctx, q, wsID, op)
	case synctypes.EntityTypePlan:
		return plan.ApplyPush(ctx, q, wsID, op)
	case synctypes.EntityTypeRoom:
		return room.ApplyPush(ctx, q, wsID, op)
	case synctypes.EntityTypeWall:
		return wall.ApplyPush(ctx, q, wsID, op)
	case synctypes.EntityTypePhoto:
		return photo.ApplyPush(ctx, q, wsID, op)
	default:
		return syncdomain.PushStepResult{
			PushError: &synctypes.PushError{Reason: "unknown", Message: "unsupported entityType"},
		}
	}
}
