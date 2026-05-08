package photos

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type PhotoPresigner interface {
	PresignedPut(ctx context.Context, photoID, contentType string) (uploadURL string, headers map[string]string, expiresAtMs int64, err error)
}

var allowedPhotoContentTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/heic": {},
	"image/heif": {},
}

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
	presigner PhotoPresigner
}

func Register(api huma.API, presigner PhotoPresigner) {
	h := &Handler{presigner: presigner}
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
	parsedID, err := uuid.Parse(photoID)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("photoId must be a valid UUID")
	}
	if parsedID.Version() != 7 {
		return nil, huma.Error422UnprocessableEntity("photoId must be a UUID v7")
	}
	photoID = parsedID.String()

	if contentType == "" {
		return nil, huma.Error422UnprocessableEntity("contentType must not be empty")
	}
	if _, ok := allowedPhotoContentTypes[contentType]; !ok {
		return nil, huma.Error422UnprocessableEntity("contentType must be an allowed image type")
	}

	uploadURL, headers, expiresAtMs, err := h.presigner.PresignedPut(ctx, photoID, contentType)
	if err != nil {
		log.Error().Err(err).Msg("presigned photo upload URL")
		return nil, huma.Error500InternalServerError("failed to create upload URL")
	}

	output := &UploadUrlOutput{}
	output.Body.UploadURL = uploadURL
	output.Body.Method = http.MethodPut
	output.Body.Headers = headers
	output.Body.ExpiresAt = expiresAtMs
	return output, nil
}
