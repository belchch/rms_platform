package syncdomain

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestEpochMsToTimestamptz(t *testing.T) {
	tests := []struct {
		name     string
		ms       int64
		expected time.Time
	}{
		{
			name:     "positive ms (after epoch)",
			ms:       1672531200000,
			expected: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "zero ms (exact epoch)",
			ms:       0,
			expected: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "negative ms (before epoch)",
			ms:       -1000,
			expected: time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EpochMsToTimestamptz(tt.ms)
			require.True(t, result.Valid)
			require.Equal(t, tt.expected.UnixMilli(), result.Time.UnixMilli())
		})
	}
}

func TestTimestamptzEpochMs(t *testing.T) {
	tests := []struct {
		name        string
		input       pgtype.Timestamptz
		expectedMs  int64
		expectError bool
	}{
		{
			name: "valid time after epoch",
			input: pgtype.Timestamptz{
				Time:  time.UnixMilli(1672531200000),
				Valid: true,
			},
			expectedMs:  1672531200000,
			expectError: false,
		},
		{
			name: "valid time exact epoch",
			input: pgtype.Timestamptz{
				Time:  time.UnixMilli(0),
				Valid: true,
			},
			expectedMs:  0,
			expectError: false,
		},
		{
			name: "valid time before epoch",
			input: pgtype.Timestamptz{
				Time:  time.UnixMilli(-1000),
				Valid: true,
			},
			expectedMs:  -1000,
			expectError: false,
		},
		{
			name: "invalid time",
			input: pgtype.Timestamptz{
				Valid: false,
			},
			expectedMs:  0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms, err := TimestamptzEpochMs(tt.input)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, int64(0), ms)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedMs, ms)
			}
		})
	}
}

func TestTimestamptzEpochMsPtr(t *testing.T) {
	tests := []struct {
		name       string
		input      pgtype.Timestamptz
		expectedMs *int64
	}{
		{
			name: "valid time after epoch",
			input: pgtype.Timestamptz{
				Time:  time.UnixMilli(1672531200000),
				Valid: true,
			},
			expectedMs: ptr(int64(1672531200000)),
		},
		{
			name: "valid time exact epoch",
			input: pgtype.Timestamptz{
				Time:  time.UnixMilli(0),
				Valid: true,
			},
			expectedMs: ptr(int64(0)),
		},
		{
			name: "valid time before epoch",
			input: pgtype.Timestamptz{
				Time:  time.UnixMilli(-1000),
				Valid: true,
			},
			expectedMs: ptr(int64(-1000)),
		},
		{
			name: "invalid time",
			input: pgtype.Timestamptz{
				Valid: false,
			},
			expectedMs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msPtr := TimestamptzEpochMsPtr(tt.input)
			if tt.expectedMs == nil {
				require.Nil(t, msPtr)
			} else {
				require.NotNil(t, msPtr)
				require.Equal(t, *tt.expectedMs, *msPtr)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
