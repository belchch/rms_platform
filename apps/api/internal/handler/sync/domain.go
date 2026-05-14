package sync

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

func lwwWins(clientMs int64, serverUpdated pgtype.Timestamptz) (bool, error) {
	if !serverUpdated.Valid {
		return false, fmt.Errorf("server updated_at is invalid: NOT NULL invariant violated")
	}
	return clientMs > serverUpdated.Time.UnixMilli(), nil
}

func validateWorkspace(actualWS, jwtWS string) *synctypes.PushError {
	if actualWS != jwtWS {
		return &synctypes.PushError{Reason: "forbidden", Message: "entity belongs to another workspace"}
	}
	return nil
}
