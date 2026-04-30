package sync

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func pullChangeFromSnapshot(snap synctypes.EntitySnapshot, updatedAt pgtype.Timestamptz, syncCursor int64, deletedAt pgtype.Timestamptz) (synctypes.PullChange, error) {
	ua, err := timestamptzEpochMs(updatedAt)
	if err != nil {
		return synctypes.PullChange{}, fmt.Errorf("pull updated_at: %w", err)
	}
	return synctypes.PullChange{
		EntityType: snap.EntityType,
		EntityID:   snap.EntityID,
		Payload:    snap.Payload,
		UpdatedAt:  ua,
		SyncCursor: syncCursor,
		DeletedAt:  timestamptzEpochMsPtr(deletedAt),
	}, nil
}

func appendPullChangesFromRows[S any](changes []synctypes.PullChange, rows []S, mapRow func(S) (synctypes.PullChange, error)) ([]synctypes.PullChange, error) {
	for _, row := range rows {
		c, err := mapRow(row)
		if err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, nil
}
