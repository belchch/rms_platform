package sync

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func epochMsToTimestamptz(ms int64) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.UnixMilli(ms), Valid: true}
}

func timestamptzEpochMs(t pgtype.Timestamptz) (int64, error) {
	if !t.Valid {
		return 0, fmt.Errorf("timestamptz is invalid")
	}
	return t.Time.UnixMilli(), nil
}

func timestamptzEpochMsPtr(t pgtype.Timestamptz) *int64 {
	if !t.Valid {
		return nil
	}
	ms := t.Time.UnixMilli()
	return &ms
}
