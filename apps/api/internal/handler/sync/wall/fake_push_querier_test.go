package wall

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncfakes"
)

type fakeWallPushQuerier struct {
	syncfakes.Unimplemented
	getProjectByID func(ctx context.Context, id string) (db.Project, error)
	getPlanByID    func(ctx context.Context, id string) (db.Plan, error)
	getRoomByID    func(ctx context.Context, id string) (db.Room, error)
	getWallByID    func(ctx context.Context, id string) (db.Wall, error)
	upsertWall     func(ctx context.Context, arg db.UpsertWallParams) (db.Wall, error)
	softDeleteWall func(ctx context.Context, arg db.SoftDeleteWallParams) (db.Wall, error)
}

func (f *fakeWallPushQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	if f.getProjectByID != nil {
		return f.getProjectByID(ctx, id)
	}
	panic("GetProjectByID not configured")
}

func (f *fakeWallPushQuerier) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	if f.getPlanByID != nil {
		return f.getPlanByID(ctx, id)
	}
	panic("GetPlanByID not configured")
}

func (f *fakeWallPushQuerier) GetRoomByID(ctx context.Context, id string) (db.Room, error) {
	if f.getRoomByID != nil {
		return f.getRoomByID(ctx, id)
	}
	panic("GetRoomByID not configured")
}

func (f *fakeWallPushQuerier) GetWallByID(ctx context.Context, id string) (db.Wall, error) {
	if f.getWallByID != nil {
		return f.getWallByID(ctx, id)
	}
	panic("GetWallByID not configured")
}

func (f *fakeWallPushQuerier) UpsertWall(ctx context.Context, arg db.UpsertWallParams) (db.Wall, error) {
	if f.upsertWall != nil {
		return f.upsertWall(ctx, arg)
	}
	panic("UpsertWall not configured")
}

func (f *fakeWallPushQuerier) SoftDeleteWall(ctx context.Context, arg db.SoftDeleteWallParams) (db.Wall, error) {
	if f.softDeleteWall != nil {
		return f.softDeleteWall(ctx, arg)
	}
	panic("SoftDeleteWall not configured")
}

var _ db.Querier = (*fakeWallPushQuerier)(nil)
