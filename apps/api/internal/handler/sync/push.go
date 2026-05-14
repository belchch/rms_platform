package sync

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
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

		if res.pushError != nil {
			res.pushError.ClientOpID = op.ClientOpID
			out.Body.Errors = append(out.Body.Errors, *res.pushError)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				log.Error().Err(rbErr).Str("workspaceId", wsID).Int("opIndex", i).Str("after", "pushError").Msg("sync push rollback savepoint failed")
				return nil, fmt.Errorf("sync push rollback savepoint: %w", rbErr)
			}
			continue
		}
		if res.conflict != nil {
			res.conflict.ClientOpID = op.ClientOpID
			out.Body.Conflicts = append(out.Body.Conflicts, *res.conflict)
			if _, rbErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				log.Error().Err(rbErr).Str("workspaceId", wsID).Int("opIndex", i).Str("after", "conflict").Msg("sync push rollback savepoint failed")
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
