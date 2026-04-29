package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/belchch/rms_platform/api/internal/jwtutil"
)

type noIn struct{}
type okOut struct {
	Body struct {
		Workspace string `json:"workspace"`
	}
}

func newTestRouter(t *testing.T, secret string) (http.Handler, huma.API) {
	t.Helper()
	router, api := humatest.New(t, huma.DefaultConfig("Test", "0.0.0"))
	api.UseMiddleware(BearerWorkspace(api, secret))
	return router, api
}

func TestBearerWorkspace_acceptsValidToken(t *testing.T) {
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var sawWorkspace string
	router, api := newTestRouter(t, secret)
	huma.Register(api, huma.Operation{
		OperationID: "check-ws",
		Method:      http.MethodGet,
		Path:        "/check",
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, _ *noIn) (*okOut, error) {
		o := &okOut{}
		sawWorkspace, _ = WorkspaceID(ctx)
		o.Body.Workspace = sawWorkspace
		return o, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if sawWorkspace != "ws-42" {
		t.Fatalf("workspace %q, want ws-42", sawWorkspace)
	}
}

func TestBearerWorkspace_allowsPublicOperation(t *testing.T) {
	router, api := newTestRouter(t, "secret")
	huma.Register(api, huma.Operation{
		OperationID: "public-op",
		Method:      http.MethodGet,
		Path:        "/public",
	}, func(_ context.Context, _ *noIn) (*okOut, error) {
		return &okOut{}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestBearerWorkspace_missingHeader(t *testing.T) {
	router, api := newTestRouter(t, "secret")
	huma.Register(api, huma.Operation{
		OperationID: "secured-op",
		Method:      http.MethodGet,
		Path:        "/secured",
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(_ context.Context, _ *noIn) (*okOut, error) {
		t.Fatal("handler must not be called without token")
		return nil, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/secured", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Fatalf("Content-Type %q, want application/problem+json", ct)
	}
}

func TestBearerWorkspace_invalidSecret(t *testing.T) {
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	router, api := newTestRouter(t, "wrongwrongwrongwrongwrongwrongwr")
	huma.Register(api, huma.Operation{
		OperationID: "secured-op2",
		Method:      http.MethodGet,
		Path:        "/secured2",
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(_ context.Context, _ *noIn) (*okOut, error) {
		t.Fatal("handler must not be called with wrong secret")
		return nil, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/secured2", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestParseBearerHeader_caseInsensitiveScheme(t *testing.T) {
	raw, ok := parseBearerHeader("BeAreR abc.def")
	if !ok || raw != "abc.def" {
		t.Fatalf("got %q ok=%v", raw, ok)
	}
}

func TestWorkspaceID_emptyStoredRejected(t *testing.T) {
	ctx := context.WithValue(context.Background(), workspaceIDCtxKey{}, "")
	_, ok := WorkspaceID(ctx)
	if ok {
		t.Fatal("expected empty workspace to be rejected")
	}
}
