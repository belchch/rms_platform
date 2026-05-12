package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/belchch/rms_platform/api/internal/db"
)

type fakeHierarchyQuerier struct {
	db.Querier
	getWallByID    func(ctx context.Context, id string) (db.Wall, error)
	getRoomByID    func(ctx context.Context, id string) (db.Room, error)
	getPlanByID    func(ctx context.Context, id string) (db.Plan, error)
	getProjectByID func(ctx context.Context, id string) (db.Project, error)
}

func (f *fakeHierarchyQuerier) GetWallByID(ctx context.Context, id string) (db.Wall, error) {
	if f.getWallByID != nil {
		return f.getWallByID(ctx, id)
	}
	return db.Wall{}, pgx.ErrNoRows
}

func (f *fakeHierarchyQuerier) GetRoomByID(ctx context.Context, id string) (db.Room, error) {
	if f.getRoomByID != nil {
		return f.getRoomByID(ctx, id)
	}
	return db.Room{}, pgx.ErrNoRows
}

func (f *fakeHierarchyQuerier) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	if f.getPlanByID != nil {
		return f.getPlanByID(ctx, id)
	}
	return db.Plan{}, pgx.ErrNoRows
}

func (f *fakeHierarchyQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	if f.getProjectByID != nil {
		return f.getProjectByID(ctx, id)
	}
	return db.Project{}, pgx.ErrNoRows
}

func TestWorkspaceOfWall(t *testing.T) {
	ctx := context.Background()
	expectedWorkspaceID := "ws-123"

	tests := []struct {
		name          string
		setupFake     func() *fakeHierarchyQuerier
		wantWorkspace string
		wantErr       error
	}{
		{
			name: "happy path - full hierarchy",
			setupFake: func() *fakeHierarchyQuerier {
				return &fakeHierarchyQuerier{
					getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
						return db.Wall{ID: "wall-1", RoomID: "room-1"}, nil
					},
					getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
						return db.Room{ID: "room-1", PlanID: "plan-1"}, nil
					},
					getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
						return db.Plan{ID: "plan-1", ProjectID: "proj-1"}, nil
					},
					getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
						return db.Project{ID: "proj-1", WorkspaceID: expectedWorkspaceID}, nil
					},
				}
			},
			wantWorkspace: expectedWorkspaceID,
			wantErr:       nil,
		},
		{
			name: "soft deleted entities still resolve workspace",
			setupFake: func() *fakeHierarchyQuerier {
				deletedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
				return &fakeHierarchyQuerier{
					getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
						return db.Wall{ID: "wall-1", RoomID: "room-1", DeletedAt: deletedAt}, nil
					},
					getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
						return db.Room{ID: "room-1", PlanID: "plan-1", DeletedAt: deletedAt}, nil
					},
					getPlanByID: func(ctx context.Context, id string) (db.Plan, error) {
						return db.Plan{ID: "plan-1", ProjectID: "proj-1", DeletedAt: deletedAt}, nil
					},
					getProjectByID: func(ctx context.Context, id string) (db.Project, error) {
						return db.Project{ID: "proj-1", WorkspaceID: expectedWorkspaceID, DeletedAt: deletedAt}, nil
					},
				}
			},
			wantWorkspace: expectedWorkspaceID,
			wantErr:       nil,
		},
		{
			name: "broken hierarchy - room not found",
			setupFake: func() *fakeHierarchyQuerier {
				return &fakeHierarchyQuerier{
					getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
						return db.Wall{ID: "wall-1", RoomID: "room-1"}, nil
					},
					getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
						return db.Room{}, pgx.ErrNoRows
					},
				}
			},
			wantWorkspace: "",
			wantErr:       pgx.ErrNoRows,
		},
		{
			name: "db error propagates",
			setupFake: func() *fakeHierarchyQuerier {
				return &fakeHierarchyQuerier{
					getWallByID: func(ctx context.Context, id string) (db.Wall, error) {
						return db.Wall{ID: "wall-1", RoomID: "room-1"}, nil
					},
					getRoomByID: func(ctx context.Context, id string) (db.Room, error) {
						return db.Room{}, errors.New("connection timeout")
					},
				}
			},
			wantWorkspace: "",
			wantErr:       errors.New("connection timeout"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.setupFake()
			wsID, err := workspaceOfWall(ctx, q, "wall-1")

			if tt.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErr.Error())
				require.Empty(t, wsID)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantWorkspace, wsID)
			}
		})
	}
}
