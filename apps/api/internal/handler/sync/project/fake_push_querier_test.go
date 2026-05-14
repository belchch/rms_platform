package project

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncfakes"
)

type fakeProjectPushQuerier struct {
	syncfakes.Unimplemented
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

var _ db.Querier = (*fakeProjectPushQuerier)(nil)
