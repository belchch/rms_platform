package sync

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func pullAppendWalls(changes []synctypes.PullChange, rows []db.Wall) ([]synctypes.PullChange, error) {
	return appendPullChangesFromRows(changes, rows, func(w db.Wall) (synctypes.PullChange, error) {
		snap, err := wallSnapshot(w)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeWall)).Str("entityId", w.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull wall snapshot: %w", err)
		}
		pc, err := pullChangeFromSnapshot(snap, w.UpdatedAt, w.SyncCursor, w.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeWall)).Str("entityId", w.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull wall snapshot: %w", err)
		}
		return pc, nil
	})
}
