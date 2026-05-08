//go:build integration

package photos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	miniosdk "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/belchch/rms_platform/api/internal/jwtutil"
	"github.com/belchch/rms_platform/api/internal/middleware"
	"github.com/belchch/rms_platform/api/internal/storage"
)

const integrationJWTSecret = "01234567890123456789012345678901"

func integrationPhotoID(t *testing.T) string {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

func newIntegrationAccessToken(t *testing.T) string {
	t.Helper()
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", integrationJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func requireDocker(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "version")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("integration tests need Docker (docker version: %v: %s)", err, bytes.TrimSpace(out))
	}
}

func startIntegrationMinIO(t *testing.T) (endpointHTTP, hostPort, user, pass, bucket string) {
	t.Helper()
	requireDocker(t)
	ctx := context.Background()
	c, err := tcminio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z")
	if err != nil {
		t.Fatalf("minio container: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	hp, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bID, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	bucket = fmt.Sprintf("it%s", strings.ReplaceAll(bID.String(), "-", "")[:24])
	return "http://" + hp, hp, c.Username, c.Password, bucket
}

func rawMinIOClient(t *testing.T, hostPort, access, secret string) *miniosdk.Client {
	t.Helper()
	cl, err := miniosdk.New(hostPort, &miniosdk.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return cl
}

func newPhotoUploadTestServer(t *testing.T, store *storage.MinioPhotoStore) *httptest.Server {
	t.Helper()
	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("integration", "0.0.0"))
	api.UseMiddleware(middleware.BearerWorkspace(api, integrationJWTSecret))
	Register(api, store)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts
}

func postUploadURL(t *testing.T, baseURL, token, photoID string) *http.Response {
	t.Helper()
	body := fmt.Sprintf(`{"photoId":%q,"contentType":"image/jpeg"}`, photoID)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/photos/upload-url", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type uploadURLBody struct {
	UploadURL string            `json:"uploadUrl"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt int64             `json:"expiresAt"`
}

func decodeUploadURLBody(t *testing.T, resp *http.Response) uploadURLBody {
	t.Helper()
	defer resp.Body.Close()
	var out uploadURLBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestIntegration_upload_presignedPut_objectExists(t *testing.T) {
	ctx := context.Background()
	endpointHTTP, hostPort, user, pass, bucket := startIntegrationMinIO(t)

	raw := rawMinIOClient(t, hostPort, user, pass)
	exists, err := raw.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("bucket should not exist before EnsureBucket")
	}

	store, err := storage.NewMinioPhotoStore(endpointHTTP, "", user, pass, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	ts := newPhotoUploadTestServer(t, store)
	token := newIntegrationAccessToken(t)
	photoID := integrationPhotoID(t)

	resp := postUploadURL(t, ts.URL, token, photoID)
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload-url: status %d: %s", resp.StatusCode, b)
	}
	u := decodeUploadURLBody(t, resp)

	payload := []byte{0xff, 0xd8, 0xff, 0xd9}
	putReq, err := http.NewRequest(http.MethodPut, u.UploadURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range u.Headers {
		putReq.Header.Set(k, v)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT object: status %d", putResp.StatusCode)
	}

	_, err = raw.StatObject(ctx, bucket, "photos/"+photoID, miniosdk.StatObjectOptions{})
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
}

func TestIntegration_ensureBucket_createsBucket(t *testing.T) {
	ctx := context.Background()
	endpointHTTP, hostPort, user, pass, bucket := startIntegrationMinIO(t)
	raw := rawMinIOClient(t, hostPort, user, pass)

	exists, err := raw.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected bucket to be missing before EnsureBucket")
	}

	store, err := storage.NewMinioPhotoStore(endpointHTTP, "", user, pass, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	exists, err = raw.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected bucket after EnsureBucket")
	}
}

func TestIntegration_presignedURL_expires(t *testing.T) {
	ctx := context.Background()
	endpointHTTP, _, user, pass, bucket := startIntegrationMinIO(t)

	store, err := storage.NewMinioPhotoStoreWithPresignTTL(endpointHTTP, "", user, pass, bucket, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	ts := newPhotoUploadTestServer(t, store)
	token := newIntegrationAccessToken(t)
	photoID := integrationPhotoID(t)

	resp := postUploadURL(t, ts.URL, token, photoID)
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload-url: status %d: %s", resp.StatusCode, b)
	}
	u := decodeUploadURLBody(t, resp)

	time.Sleep(3 * time.Second)

	putReq, err := http.NewRequest(http.MethodPut, u.UploadURL, bytes.NewReader([]byte{1, 2, 3}))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range u.Headers {
		putReq.Header.Set(k, v)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expired presign PUT: status %d, want %d", putResp.StatusCode, http.StatusForbidden)
	}
}

func TestIntegration_samePhotoId_twiceUploadUrlsOverwrite(t *testing.T) {
	ctx := context.Background()
	endpointHTTP, hostPort, user, pass, bucket := startIntegrationMinIO(t)

	store, err := storage.NewMinioPhotoStore(endpointHTTP, "", user, pass, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	raw := rawMinIOClient(t, hostPort, user, pass)
	ts := newPhotoUploadTestServer(t, store)
	token := newIntegrationAccessToken(t)
	photoID := integrationPhotoID(t)

	resp1 := postUploadURL(t, ts.URL, token, photoID)
	if resp1.StatusCode != http.StatusOK {
		defer resp1.Body.Close()
		b, _ := io.ReadAll(resp1.Body)
		t.Fatalf("first upload-url: status %d: %s", resp1.StatusCode, b)
	}
	u1 := decodeUploadURLBody(t, resp1)

	resp2 := postUploadURL(t, ts.URL, token, photoID)
	if resp2.StatusCode != http.StatusOK {
		defer resp2.Body.Close()
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("second upload-url: status %d: %s", resp2.StatusCode, b)
	}
	u2 := decodeUploadURLBody(t, resp2)

	first := []byte("first-bytes")
	put1, err := http.NewRequest(http.MethodPut, u1.UploadURL, bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range u1.Headers {
		put1.Header.Set(k, v)
	}
	pr1, err := http.DefaultClient.Do(put1)
	if err != nil {
		t.Fatal(err)
	}
	pr1.Body.Close()
	if pr1.StatusCode != http.StatusOK {
		t.Fatalf("first PUT: %d", pr1.StatusCode)
	}

	second := []byte("second-bytes")
	put2, err := http.NewRequest(http.MethodPut, u2.UploadURL, bytes.NewReader(second))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range u2.Headers {
		put2.Header.Set(k, v)
	}
	pr2, err := http.DefaultClient.Do(put2)
	if err != nil {
		t.Fatal(err)
	}
	pr2.Body.Close()
	if pr2.StatusCode != http.StatusOK {
		t.Fatalf("second PUT: %d", pr2.StatusCode)
	}

	obj, err := raw.GetObject(ctx, bucket, "photos/"+photoID, miniosdk.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer obj.Close()
	got, err := io.ReadAll(obj)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(second) {
		t.Fatalf("object body %q, want %q", got, second)
	}
}
