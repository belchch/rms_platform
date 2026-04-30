package sync

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/db"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func pullAppendRooms(changes []synctypes.PullChange, rows []db.Room) ([]synctypes.PullChange, error) {
	return appendPullChangesFromRows(changes, rows, func(r db.Room) (synctypes.PullChange, error) {
		snap, err := roomSnapshot(r)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeRoom)).Str("entityId", r.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull room snapshot: %w", err)
		}
		pc, err := pullChangeFromSnapshot(snap, r.UpdatedAt, r.SyncCursor, r.DeletedAt)
		if err != nil {
			log.Error().Err(err).Str("entityType", string(synctypes.EntityTypeRoom)).Str("entityId", r.ID).Msg("sync pull snapshot failed")
			return synctypes.PullChange{}, fmt.Errorf("sync pull room snapshot: %w", err)
		}
		return pc, nil
	})
}
