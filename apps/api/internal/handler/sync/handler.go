package sync

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"

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

var bearerAuth = []map[string][]string{{"bearerAuth": {}}}

func Register(api huma.API, pool *pgxpool.Pool) {
	h := &handler{pool: pool}

	huma.Register(api, huma.Operation{
		OperationID: "sync-pull",
		Method:      http.MethodGet,
		Path:        "/api/v1/sync/pull",
		Summary:     "Pull changes from server",
		Tags:        []string{"sync"},
		Security:    bearerAuth,
	}, h.pull)

	huma.Register(api, huma.Operation{
		OperationID: "sync-push",
		Method:      http.MethodPost,
		Path:        "/api/v1/sync/push",
		Summary:     "Push local changes to server",
		Tags:        []string{"sync"},
		Security:    bearerAuth,
	}, h.push)
}
