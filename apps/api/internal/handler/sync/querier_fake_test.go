package sync

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
)

type unimplementedQuerier struct{}

func (unimplementedQuerier) CreatePhotoable(ctx context.Context, arg db.CreatePhotoableParams) (db.Photoable, error) {
	panic("unexpected CreatePhotoable")
}

func (unimplementedQuerier) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) error {
	panic("unexpected CreateRefreshToken")
}

func (unimplementedQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	panic("unexpected CreateUser")
}

func (unimplementedQuerier) CreateWorkspace(ctx context.Context, arg db.CreateWorkspaceParams) (db.Workspace, error) {
	panic("unexpected CreateWorkspace")
}

func (unimplementedQuerier) DeleteRefreshToken(ctx context.Context, id string) error {
	panic("unexpected DeleteRefreshToken")
}

func (unimplementedQuerier) GetPhotoByID(ctx context.Context, id string) (db.Photo, error) {
	panic("unexpected GetPhotoByID")
}

func (unimplementedQuerier) GetPhotoableByID(ctx context.Context, id string) (db.Photoable, error) {
	panic("unexpected GetPhotoableByID")
}

func (unimplementedQuerier) GetPhotoableByOwner(ctx context.Context, arg db.GetPhotoableByOwnerParams) (db.Photoable, error) {
	panic("unexpected GetPhotoableByOwner")
}

func (unimplementedQuerier) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	panic("unexpected GetPlanByID")
}

func (unimplementedQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	panic("unexpected GetProjectByID")
}

func (unimplementedQuerier) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	panic("unexpected GetRefreshTokenByHash")
}

func (unimplementedQuerier) GetRefreshTokenByHashForUpdate(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	panic("unexpected GetRefreshTokenByHashForUpdate")
}

func (unimplementedQuerier) GetRoomByID(ctx context.Context, id string) (db.Room, error) {
	panic("unexpected GetRoomByID")
}

func (unimplementedQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	panic("unexpected GetUserByEmail")
}

func (unimplementedQuerier) GetUserByID(ctx context.Context, id string) (db.User, error) {
	panic("unexpected GetUserByID")
}

func (unimplementedQuerier) GetWallByID(ctx context.Context, id string) (db.Wall, error) {
	panic("unexpected GetWallByID")
}

func (unimplementedQuerier) GetWorkspaceByID(ctx context.Context, id string) (db.Workspace, error) {
	panic("unexpected GetWorkspaceByID")
}

func (unimplementedQuerier) GetWorkspaceByOwnerID(ctx context.Context, ownerID string) (db.Workspace, error) {
	panic("unexpected GetWorkspaceByOwnerID")
}

func (unimplementedQuerier) ListPhotosSince(ctx context.Context, arg db.ListPhotosSinceParams) ([]db.ListPhotosSinceRow, error) {
	panic("unexpected ListPhotosSince")
}

func (unimplementedQuerier) ListPlansSince(ctx context.Context, arg db.ListPlansSinceParams) ([]db.Plan, error) {
	panic("unexpected ListPlansSince")
}

func (unimplementedQuerier) ListProjectsSince(ctx context.Context, arg db.ListProjectsSinceParams) ([]db.Project, error) {
	panic("unexpected ListProjectsSince")
}

func (unimplementedQuerier) ListRoomsSince(ctx context.Context, arg db.ListRoomsSinceParams) ([]db.Room, error) {
	panic("unexpected ListRoomsSince")
}

func (unimplementedQuerier) ListWallsSince(ctx context.Context, arg db.ListWallsSinceParams) ([]db.Wall, error) {
	panic("unexpected ListWallsSince")
}

func (unimplementedQuerier) SetPhotoRemoteURL(ctx context.Context, arg db.SetPhotoRemoteURLParams) error {
	panic("unexpected SetPhotoRemoteURL")
}

func (unimplementedQuerier) SoftDeletePhoto(ctx context.Context, arg db.SoftDeletePhotoParams) (db.Photo, error) {
	panic("unexpected SoftDeletePhoto")
}

func (unimplementedQuerier) SoftDeletePlan(ctx context.Context, arg db.SoftDeletePlanParams) (db.Plan, error) {
	panic("unexpected SoftDeletePlan")
}

func (unimplementedQuerier) SoftDeleteProject(ctx context.Context, arg db.SoftDeleteProjectParams) (db.Project, error) {
	panic("unexpected SoftDeleteProject")
}

func (unimplementedQuerier) SoftDeleteRoom(ctx context.Context, arg db.SoftDeleteRoomParams) (db.Room, error) {
	panic("unexpected SoftDeleteRoom")
}

func (unimplementedQuerier) SoftDeleteWall(ctx context.Context, arg db.SoftDeleteWallParams) (db.Wall, error) {
	panic("unexpected SoftDeleteWall")
}

func (unimplementedQuerier) UpsertPhoto(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error) {
	panic("unexpected UpsertPhoto")
}

func (unimplementedQuerier) UpsertPhotoableByOwner(ctx context.Context, arg db.UpsertPhotoableByOwnerParams) (db.Photoable, error) {
	panic("unexpected UpsertPhotoableByOwner")
}

func (unimplementedQuerier) UpsertPlan(ctx context.Context, arg db.UpsertPlanParams) (db.Plan, error) {
	panic("unexpected UpsertPlan")
}

func (unimplementedQuerier) UpsertProject(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error) {
	panic("unexpected UpsertProject")
}

func (unimplementedQuerier) UpsertRoom(ctx context.Context, arg db.UpsertRoomParams) (db.Room, error) {
	panic("unexpected UpsertRoom")
}

func (unimplementedQuerier) UpsertWall(ctx context.Context, arg db.UpsertWallParams) (db.Wall, error) {
	panic("unexpected UpsertWall")
}

type fakeProjectPushQuerier struct {
	unimplementedQuerier
	getProjectByID    func(ctx context.Context, id string) (db.Project, error)
	upsertProject     func(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error)
	softDeleteProject func(ctx context.Context, arg db.SoftDeleteProjectParams) (db.Project, error)
}

func (f *fakeProjectPushQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	if f.getProjectByID != nil {
		return f.getProjectByID(ctx, id)
	}
	panic("GetProjectByID not configured")
}

func (f *fakeProjectPushQuerier) UpsertProject(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error) {
	if f.upsertProject != nil {
		return f.upsertProject(ctx, arg)
	}
	panic("UpsertProject not configured")
}

func (f *fakeProjectPushQuerier) SoftDeleteProject(ctx context.Context, arg db.SoftDeleteProjectParams) (db.Project, error) {
	if f.softDeleteProject != nil {
		return f.softDeleteProject(ctx, arg)
	}
	panic("SoftDeleteProject not configured")
}

type fakePlanPushQuerier struct {
	unimplementedQuerier
	getProjectByID func(ctx context.Context, id string) (db.Project, error)
	getPlanByID    func(ctx context.Context, id string) (db.Plan, error)
	upsertPlan     func(ctx context.Context, arg db.UpsertPlanParams) (db.Plan, error)
	softDeletePlan func(ctx context.Context, arg db.SoftDeletePlanParams) (db.Plan, error)
}

func (f *fakePlanPushQuerier) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	if f.getProjectByID != nil {
		return f.getProjectByID(ctx, id)
	}
	panic("GetProjectByID not configured")
}

func (f *fakePlanPushQuerier) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	if f.getPlanByID != nil {
		return f.getPlanByID(ctx, id)
	}
	panic("GetPlanByID not configured")
}

func (f *fakePlanPushQuerier) UpsertPlan(ctx context.Context, arg db.UpsertPlanParams) (db.Plan, error) {
	if f.upsertPlan != nil {
		return f.upsertPlan(ctx, arg)
	}
	panic("UpsertPlan not configured")
}

func (f *fakePlanPushQuerier) SoftDeletePlan(ctx context.Context, arg db.SoftDeletePlanParams) (db.Plan, error) {
	if f.softDeletePlan != nil {
		return f.softDeletePlan(ctx, arg)
	}
	panic("SoftDeletePlan not configured")
}

type fakeRoomPushQuerier struct {
	unimplementedQuerier
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

type fakeWallPushQuerier struct {
	unimplementedQuerier
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

type fakePhotoPushQuerier struct {
	unimplementedQuerier
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

var _ db.Querier = (*fakeProjectPushQuerier)(nil)
var _ db.Querier = (*fakePlanPushQuerier)(nil)
var _ db.Querier = (*fakeRoomPushQuerier)(nil)
var _ db.Querier = (*fakeWallPushQuerier)(nil)
var _ db.Querier = (*fakePhotoPushQuerier)(nil)
