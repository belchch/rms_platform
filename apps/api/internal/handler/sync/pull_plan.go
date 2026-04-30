package sync

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func pullAppendPlans(changes []synctypes.PullChange, rows []db.Plan) ([]synctypes.PullChange, error) {
	return appendPullChangesFromRows(changes, rows, func(p db.Plan) (synctypes.PullChange, error) {
		snap, err := planSnapshot(p)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypePlan)).Str("entityId", p.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull plan snapshot: %w", err)
		}
		pc, err := pullChangeFromSnapshot(snap, p.UpdatedAt, p.SyncCursor, p.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypePlan)).Str("entityId", p.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull plan snapshot: %w", err)
		}
		return pc, nil
	})
}
