package plan

import (
	"context"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/handler/sync/syncfakes"
)

type fakePlanPushQuerier struct {
	syncfakes.Unimplemented
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

var _ db.Querier = (*fakePlanPushQuerier)(nil)
