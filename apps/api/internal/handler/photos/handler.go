package photos

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

type UploadUrlInput struct {
	Body struct {
		PhotoID     string `json:"photoId"`
		ContentType string `json:"contentType"`
	}
}

type UploadUrlOutput struct {
	Body struct {
		UploadURL string            `json:"uploadUrl"`
		Method    string            `json:"method"`
		Headers   map[string]string `json:"headers"`
		ExpiresAt int64             `json:"expiresAt"`
	}
}

func Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-photo-upload-url",
		Method:      "POST",
		Path:        "/api/v1/photos/upload-url",
		Summary:     "Get pre-signed PUT URL for photo upload",
		Tags:        []string{"photos"},
	}, uploadUrl)
}

func uploadUrl(_ context.Context, _ *UploadUrlInput) (*UploadUrlOutput, error) {
	output := &UploadUrlOutput{}
	output.Body.UploadURL = "todo"
	output.Body.Method = "PUT"
	output.Body.Headers = map[string]string{}
	output.Body.ExpiresAt = 0
	return output, nil
}
