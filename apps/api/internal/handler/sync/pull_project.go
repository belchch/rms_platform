package sync

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func pullAppendProjects(changes []synctypes.PullChange, rows []db.Project) ([]synctypes.PullChange, error) {
	return appendPullChangesFromRows(changes, rows, func(p db.Project) (synctypes.PullChange, error) {
		snap, err := projectSnapshot(p)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeProject)).Str("entityId", p.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull project snapshot: %w", err)
		}
		pc, err := pullChangeFromSnapshot(snap, p.UpdatedAt, p.SyncCursor, p.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeProject)).Str("entityId", p.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull project snapshot: %w", err)
		}
		return pc, nil
	})
}
