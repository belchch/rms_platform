package sync

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/middleware"
	synctypes "github.com/belchch/rms_platform/api/internal/sync"
)

type PullInput struct {
	Since int64 `query:"since" doc:"Server cursor from previous pull"`
}

type PullOutput struct {
	Body struct {
		Cursor  int64                  `json:"cursor"`
		Changes []synctypes.PullChange `json:"changes"`
	}
}

type PushInput struct {
	Body struct {
		Operations []synctypes.PushOperation `json:"operations"`
	}
}

type PushOutput struct {
	Body struct {
		Cursor    int64                    `json:"cursor"`
		Applied   []string                 `json:"applied"`
		Conflicts []synctypes.PushConflict `json:"conflicts"`
		Errors    []synctypes.PushError    `json:"errors"`
	}
}

func Register(api huma.API, q *db.Queries, pool *pgxpool.Pool) {
	huma.Register(api, huma.Operation{
		OperationID: "sync-pull",
		Method:      "GET",
		Path:        "/api/v1/sync/pull",
		Summary:     "Pull changes from server",
		Tags:        []string{"sync"},
	}, pull)

	huma.Register(api, huma.Operation{
		OperationID: "sync-push",
		Method:      "POST",
		Path:        "/api/v1/sync/push",
		Summary:     "Push local changes to server",
		Tags:        []string{"sync"},
	}, push)
}

func pull(ctx context.Context, _ *PullInput) (*PullOutput, error) {
	if _, ok := middleware.WorkspaceID(ctx); !ok {
		return nil, huma.NewError(http.StatusInternalServerError, "missing workspace context")
	}
	output := &PullOutput{}
	output.Body.Cursor = 0
	output.Body.Changes = []synctypes.PullChange{}
	return output, nil
}

func push(ctx context.Context, _ *PushInput) (*PushOutput, error) {
	if _, ok := middleware.WorkspaceID(ctx); !ok {
		return nil, huma.NewError(http.StatusInternalServerError, "missing workspace context")
	}
	output := &PushOutput{}
	output.Body.Cursor = 0
	output.Body.Applied = []string{}
	output.Body.Conflicts = []synctypes.PushConflict{}
	output.Body.Errors = []synctypes.PushError{}
	return output, nil
}
