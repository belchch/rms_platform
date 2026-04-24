package sync

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

type PullInput struct {
	Since int64 `query:"since" doc:"Server cursor from previous pull"`
}

type PullOutput struct {
	Body struct {
		Cursor  int64         `json:"cursor"`
		Changes []interface{} `json:"changes"`
	}
}

type PushInput struct {
	Body struct {
		Operations []interface{} `json:"operations"`
	}
}

type PushOutput struct {
	Body struct {
		Cursor int64 `json:"cursor"`
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

func pull(_ context.Context, input *PullInput) (*PullOutput, error) {
	output := &PullOutput{}
	output.Body.Cursor = 0
	output.Body.Changes = []interface{}{}
	return output, nil
}

func push(_ context.Context, input *PushInput) (*PushOutput, error) {
	output := &PushOutput{}
	output.Body.Cursor = 0
	return output, nil
}
