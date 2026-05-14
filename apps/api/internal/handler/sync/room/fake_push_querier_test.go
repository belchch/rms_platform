package room

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncfakes"
)

type fakeRoomPushQuerier struct {
	syncfakes.Unimplemented
	getProjectByID func(ctx context.Context, id string) (db.Project, error)
	getPlanByID    func(ctx context.Context, id string) (db.Plan, error)
	getRoomByID    func(ctx context.Context, id string) (db.Room, error)
	upsertRoom     func(ctx context.Context, arg db.UpsertRoomParams) (db.Room, error)
	softDeleteRoom func(ctx context.Context, arg db.SoftDeleteRoomParams) (db.Room, error)
}

func (f *fakeRoomPushQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	if f.getProjectByID != nil {
		return f.getProjectByID(ctx, id)
	}
	panic("GetProjectByID not configured")
}

func (f *fakeRoomPushQuerier) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	if f.getPlanByID != nil {
		return f.getPlanByID(ctx, id)
	}
	panic("GetPlanByID not configured")
}

func (f *fakeRoomPushQuerier) GetRoomByID(ctx context.Context, id string) (db.Room, error) {
	if f.getRoomByID != nil {
		return f.getRoomByID(ctx, id)
	}
	panic("GetRoomByID not configured")
}

func (f *fakeRoomPushQuerier) UpsertRoom(ctx context.Context, arg db.UpsertRoomParams) (db.Room, error) {
	if f.upsertRoom != nil {
		return f.upsertRoom(ctx, arg)
	}
	panic("UpsertRoom not configured")
}

func (f *fakeRoomPushQuerier) SoftDeleteRoom(ctx context.Context, arg db.SoftDeleteRoomParams) (db.Room, error) {
	if f.softDeleteRoom != nil {
		return f.softDeleteRoom(ctx, arg)
	}
	panic("SoftDeleteRoom not configured")
}

var _ db.Querier = (*fakeRoomPushQuerier)(nil)
