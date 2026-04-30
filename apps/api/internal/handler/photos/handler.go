package photos

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/belchch/rms_platform/api/internal/storage"
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

type Handler struct {
	store storage.PhotoUploader
}

func Register(api huma.API, store storage.PhotoUploader) {
	h := &Handler{store: store}
	huma.Register(api, huma.Operation{
		OperationID: "get-photo-upload-url",
		Method:      http.MethodPost,
		Path:        "/api/v1/photos/upload-url",
		Summary:     "Get pre-signed PUT URL for photo upload",
		Tags:        []string{"photos"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, h.uploadUrl)
}

func (h *Handler) uploadUrl(ctx context.Context, input *UploadUrlInput) (*UploadUrlOutput, error) {
	photoID := strings.TrimSpace(input.Body.PhotoID)
	contentType := strings.TrimSpace(input.Body.ContentType)
	if photoID == "" {
		return nil, huma.Error422UnprocessableEntity("photoId must not be empty")
	}
	if contentType == "" {
		return nil, huma.Error422UnprocessableEntity("contentType must not be empty")
	}

	uploadURL, headers, expiresAtMs, err := h.store.PresignedPut(ctx, photoID, contentType)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	output := &UploadUrlOutput{}
	output.Body.UploadURL = uploadURL
	output.Body.Method = http.MethodPut
	output.Body.Headers = headers
	output.Body.ExpiresAt = expiresAtMs
	return output, nil
}
