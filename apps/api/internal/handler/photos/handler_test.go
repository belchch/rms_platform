package photos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/belchch/rms_platform/api/internal/jwtutil"
	"github.com/belchch/rms_platform/api/internal/middleware"
)

type stubPhotoStore struct {
	uploadURL string
	err       error
}

func (s *stubPhotoStore) EnsureBucket(context.Context) error { return nil }

func (s *stubPhotoStore) PresignedPut(_ context.Context, photoID, contentType string) (string, map[string]string, int64, error) {
	if s.err != nil {
		return "", nil, 0, s.err
	}
	u := s.uploadURL + photoID
	return u, map[string]string{"Content-Type": contentType}, time.Now().Add(time.Hour).UnixMilli(), nil
}

func TestUploadUrl_happyPath(t *testing.T) {
	t.Parallel()
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	base := "http://localhost:9000/test-bucket/photos/"
	store := &stubPhotoStore{uploadURL: base}

	router, api := humatest.New(t, huma.DefaultConfig("Test", "0.0.0"))
	api.UseMiddleware(middleware.BearerWorkspace(api, secret))
	Register(api, store)

	body := `{"photoId":"01JTEST123456789ABC","contentType":"image/jpeg"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		UploadURL string            `json:"uploadUrl"`
		Method    string            `json:"method"`
		Headers   map[string]string `json:"headers"`
		ExpiresAt int64             `json:"expiresAt"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Method != http.MethodPut {
		t.Fatalf("method %q", resp.Method)
	}
	if resp.Headers["Content-Type"] != "image/jpeg" {
		t.Fatalf("headers %#v", resp.Headers)
	}
	if resp.UploadURL == "" {
		t.Fatal("empty uploadUrl")
	}
	if !strings.Contains(resp.UploadURL, "01JTEST123456789ABC") {
		t.Fatalf("uploadUrl %q", resp.UploadURL)
	}
	if resp.ExpiresAt <= 1e12 {
		t.Fatalf("expiresAt %d", resp.ExpiresAt)
	}
}

func TestUploadUrl_requiresAuth(t *testing.T) {
	t.Parallel()
	store := &stubPhotoStore{uploadURL: "http://x/"}
	router, api := humatest.New(t, huma.DefaultConfig("Test", "0.0.0"))
	api.UseMiddleware(middleware.BearerWorkspace(api, "01234567890123456789012345678901"))
	Register(api, store)

	body := `{"photoId":"01JTEST123456789ABC","contentType":"image/jpeg"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestUploadUrl_emptyPhotoId(t *testing.T) {
	t.Parallel()
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &stubPhotoStore{uploadURL: "http://x/"}
	router, api := humatest.New(t, huma.DefaultConfig("Test", "0.0.0"))
	api.UseMiddleware(middleware.BearerWorkspace(api, secret))
	Register(api, store)

	body := `{"photoId":"   ","contentType":"image/jpeg"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUploadUrl_emptyContentType(t *testing.T) {
	t.Parallel()
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &stubPhotoStore{uploadURL: "http://x/"}
	router, api := humatest.New(t, huma.DefaultConfig("Test", "0.0.0"))
	api.UseMiddleware(middleware.BearerWorkspace(api, secret))
	Register(api, store)

	body := `{"photoId":"01JTEST123456789ABC","contentType":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUploadUrl_storageError(t *testing.T) {
	t.Parallel()
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &stubPhotoStore{err: errors.New("s3 unavailable")}
	router, api := humatest.New(t, huma.DefaultConfig("Test", "0.0.0"))
	api.UseMiddleware(middleware.BearerWorkspace(api, secret))
	Register(api, store)

	body := `{"photoId":"01JTEST123456789ABC","contentType":"image/jpeg"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/photos/upload-url", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
}
