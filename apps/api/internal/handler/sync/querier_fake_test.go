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
	getProjectByID func(ctx context.Context, id string) (db.Project, error)
	upsertProject  func(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error)
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

var _ db.Querier = (*fakeProjectPushQuerier)(nil)
