package photo

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncfakes"
)

type fakePhotoPushQuerier struct {
	syncfakes.Unimplemented
	getProjectByID         func(ctx context.Context, id string) (db.Project, error)
	getPlanByID            func(ctx context.Context, id string) (db.Plan, error)
	getRoomByID            func(ctx context.Context, id string) (db.Room, error)
	getWallByID            func(ctx context.Context, id string) (db.Wall, error)
	getPhotoByID           func(ctx context.Context, id string) (db.Photo, error)
	getPhotoableByID       func(ctx context.Context, id string) (db.Photoable, error)
	upsertPhotoableByOwner func(ctx context.Context, arg db.UpsertPhotoableByOwnerParams) (db.Photoable, error)
	upsertPhoto            func(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error)
	softDeletePhoto        func(ctx context.Context, arg db.SoftDeletePhotoParams) (db.Photo, error)
}

func (f *fakePhotoPushQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	if f.getProjectByID != nil {
		return f.getProjectByID(ctx, id)
	}
	panic("GetProjectByID not configured")
}

func (f *fakePhotoPushQuerier) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	if f.getPlanByID != nil {
		return f.getPlanByID(ctx, id)
	}
	panic("GetPlanByID not configured")
}

func (f *fakePhotoPushQuerier) GetRoomByID(ctx context.Context, id string) (db.Room, error) {
	if f.getRoomByID != nil {
		return f.getRoomByID(ctx, id)
	}
	panic("GetRoomByID not configured")
}

func (f *fakePhotoPushQuerier) GetWallByID(ctx context.Context, id string) (db.Wall, error) {
	if f.getWallByID != nil {
		return f.getWallByID(ctx, id)
	}
	panic("GetWallByID not configured")
}

func (f *fakePhotoPushQuerier) GetPhotoByID(ctx context.Context, id string) (db.Photo, error) {
	if f.getPhotoByID != nil {
		return f.getPhotoByID(ctx, id)
	}
	panic("GetPhotoByID not configured")
}

func (f *fakePhotoPushQuerier) GetPhotoableByID(ctx context.Context, id string) (db.Photoable, error) {
	if f.getPhotoableByID != nil {
		return f.getPhotoableByID(ctx, id)
	}
	panic("GetPhotoableByID not configured")
}

func (f *fakePhotoPushQuerier) UpsertPhotoableByOwner(ctx context.Context, arg db.UpsertPhotoableByOwnerParams) (db.Photoable, error) {
	if f.upsertPhotoableByOwner != nil {
		return f.upsertPhotoableByOwner(ctx, arg)
	}
	panic("UpsertPhotoableByOwner not configured")
}

func (f *fakePhotoPushQuerier) UpsertPhoto(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error) {
	if f.upsertPhoto != nil {
		return f.upsertPhoto(ctx, arg)
	}
	panic("UpsertPhoto not configured")
}

func (f *fakePhotoPushQuerier) SoftDeletePhoto(ctx context.Context, arg db.SoftDeletePhotoParams) (db.Photo, error) {
	if f.softDeletePhoto != nil {
		return f.softDeletePhoto(ctx, arg)
	}
	panic("SoftDeletePhoto not configured")
}

var _ db.Querier = (*fakePhotoPushQuerier)(nil)
