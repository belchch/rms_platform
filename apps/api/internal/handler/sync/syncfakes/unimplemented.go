package syncfakes

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
)

type Unimplemented struct{}

func (Unimplemented) CreatePhotoable(ctx context.Context, arg db.CreatePhotoableParams) (db.Photoable, error) {
	panic("unexpected CreatePhotoable")
}

func (Unimplemented) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) error {
	panic("unexpected CreateRefreshToken")
}

func (Unimplemented) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	panic("unexpected CreateUser")
}

func (Unimplemented) CreateWorkspace(ctx context.Context, arg db.CreateWorkspaceParams) (db.Workspace, error) {
	panic("unexpected CreateWorkspace")
}

func (Unimplemented) DeleteRefreshToken(ctx context.Context, id string) error {
	panic("unexpected DeleteRefreshToken")
}

func (Unimplemented) GetPhotoByID(ctx context.Context, id string) (db.Photo, error) {
	panic("unexpected GetPhotoByID")
}

func (Unimplemented) GetPhotoableByID(ctx context.Context, id string) (db.Photoable, error) {
	panic("unexpected GetPhotoableByID")
}

func (Unimplemented) GetPhotoableByOwner(ctx context.Context, arg db.GetPhotoableByOwnerParams) (db.Photoable, error) {
	panic("unexpected GetPhotoableByOwner")
}

func (Unimplemented) GetPlanByID(ctx context.Context, id string) (db.Plan, error) {
	panic("unexpected GetPlanByID")
}

func (Unimplemented) GetProjectByID(ctx context.Context, id string) (db.Project, error) {
	panic("unexpected GetProjectByID")
}

func (Unimplemented) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	panic("unexpected GetRefreshTokenByHash")
}

func (Unimplemented) GetRefreshTokenByHashForUpdate(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	panic("unexpected GetRefreshTokenByHashForUpdate")
}

func (Unimplemented) GetRoomByID(ctx context.Context, id string) (db.Room, error) {
	panic("unexpected GetRoomByID")
}

func (Unimplemented) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	panic("unexpected GetUserByEmail")
}

func (Unimplemented) GetUserByID(ctx context.Context, id string) (db.User, error) {
	panic("unexpected GetUserByID")
}

func (Unimplemented) GetWallByID(ctx context.Context, id string) (db.Wall, error) {
	panic("unexpected GetWallByID")
}

func (Unimplemented) GetWorkspaceByID(ctx context.Context, id string) (db.Workspace, error) {
	panic("unexpected GetWorkspaceByID")
}

func (Unimplemented) GetWorkspaceByOwnerID(ctx context.Context, ownerID string) (db.Workspace, error) {
	panic("unexpected GetWorkspaceByOwnerID")
}

func (Unimplemented) ListPhotosSince(ctx context.Context, arg db.ListPhotosSinceParams) ([]db.ListPhotosSinceRow, error) {
	panic("unexpected ListPhotosSince")
}

func (Unimplemented) ListPlansSince(ctx context.Context, arg db.ListPlansSinceParams) ([]db.Plan, error) {
	panic("unexpected ListPlansSince")
}

func (Unimplemented) ListProjectsSince(ctx context.Context, arg db.ListProjectsSinceParams) ([]db.Project, error) {
	panic("unexpected ListProjectsSince")
}

func (Unimplemented) ListRoomsSince(ctx context.Context, arg db.ListRoomsSinceParams) ([]db.Room, error) {
	panic("unexpected ListRoomsSince")
}

func (Unimplemented) ListWallsSince(ctx context.Context, arg db.ListWallsSinceParams) ([]db.Wall, error) {
	panic("unexpected ListWallsSince")
}

func (Unimplemented) SetPhotoRemoteURL(ctx context.Context, arg db.SetPhotoRemoteURLParams) error {
	panic("unexpected SetPhotoRemoteURL")
}

func (Unimplemented) SoftDeletePhoto(ctx context.Context, arg db.SoftDeletePhotoParams) (db.Photo, error) {
	panic("unexpected SoftDeletePhoto")
}

func (Unimplemented) SoftDeletePlan(ctx context.Context, arg db.SoftDeletePlanParams) (db.Plan, error) {
	panic("unexpected SoftDeletePlan")
}

func (Unimplemented) SoftDeleteProject(ctx context.Context, arg db.SoftDeleteProjectParams) (db.Project, error) {
	panic("unexpected SoftDeleteProject")
}

func (Unimplemented) SoftDeleteRoom(ctx context.Context, arg db.SoftDeleteRoomParams) (db.Room, error) {
	panic("unexpected SoftDeleteRoom")
}

func (Unimplemented) SoftDeleteWall(ctx context.Context, arg db.SoftDeleteWallParams) (db.Wall, error) {
	panic("unexpected SoftDeleteWall")
}

func (Unimplemented) UpsertPhoto(ctx context.Context, arg db.UpsertPhotoParams) (db.Photo, error) {
	panic("unexpected UpsertPhoto")
}

func (Unimplemented) UpsertPhotoableByOwner(ctx context.Context, arg db.UpsertPhotoableByOwnerParams) (db.Photoable, error) {
	panic("unexpected UpsertPhotoableByOwner")
}

func (Unimplemented) UpsertPlan(ctx context.Context, arg db.UpsertPlanParams) (db.Plan, error) {
	panic("unexpected UpsertPlan")
}

func (Unimplemented) UpsertProject(ctx context.Context, arg db.UpsertProjectParams) (db.Project, error) {
	panic("unexpected UpsertProject")
}

func (Unimplemented) UpsertRoom(ctx context.Context, arg db.UpsertRoomParams) (db.Room, error) {
	panic("unexpected UpsertRoom")
}

func (Unimplemented) UpsertWall(ctx context.Context, arg db.UpsertWallParams) (db.Wall, error) {
	panic("unexpected UpsertWall")
}
