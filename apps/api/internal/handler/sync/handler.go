package sync

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

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

func Register(api huma.API) {
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

func pull(_ context.Context, _ *PullInput) (*PullOutput, error) {
	output := &PullOutput{}
	output.Body.Cursor = 0
	output.Body.Changes = []synctypes.PullChange{}
	return output, nil
}

func push(_ context.Context, _ *PushInput) (*PushOutput, error) {
	output := &PushOutput{}
	output.Body.Cursor = 0
	output.Body.Applied = []string{}
	output.Body.Conflicts = []synctypes.PushConflict{}
	output.Body.Errors = []synctypes.PushError{}
	return output, nil
}
