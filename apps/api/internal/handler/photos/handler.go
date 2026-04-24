package photos

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

type UploadInput struct {
	RawBody []byte
}

type UploadOutput struct {
	Body struct {
		ID        string `json:"id"`
		RemoteURL string `json:"remoteUrl"`
	}
}

func Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:  "upload-photo",
		Method:       "POST",
		Path:         "/api/v1/photos",
		Summary:      "Upload photo (multipart or pre-signed flow)",
		Tags:         []string{"photos"},
		DefaultStatus: 201,
	}, upload)
}

func upload(_ context.Context, input *UploadInput) (*UploadOutput, error) {
	output := &UploadOutput{}
	output.Body.ID = "todo"
	output.Body.RemoteURL = "todo"
	return output, nil
}
