package syncdomain

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestLwwWins(t *testing.T) {
	serverTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	serverMs := serverTime.UnixMilli()

	validServer := pgtype.Timestamptz{Time: serverTime, Valid: true}

	tests := []struct {
		name       string
		clientMs   int64
		server     pgtype.Timestamptz
		wantWins   bool
		wantErrMsg string
	}{
		{
			name:     "client newer than server — wins",
			clientMs: serverMs + 1,
			server:   validServer,
			wantWins: true,
		},
		{
			name:     "client older than server — loses",
			clientMs: serverMs - 1,
			server:   validServer,
			wantWins: false,
		},
		{
			name:     "client equal to server — loses (server keeps its value)",
			clientMs: serverMs,
			server:   validServer,
			wantWins: false,
		},
		{
			name:       "server updated_at is invalid — NOT NULL invariant violated",
			clientMs:   serverMs + 1,
			server:     pgtype.Timestamptz{Valid: false},
			wantErrMsg: "NOT NULL invariant violated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wins, err := LWWWins(tt.clientMs, tt.server)

			if tt.wantErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrMsg)
				require.False(t, wins)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantWins, wins)
		})
	}
}
