package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/belchch/rms_platform/api/internal/jwtutil"
)

func TestBearerWorkspace_acceptsValidToken(t *testing.T) {
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	var sawWorkspace string
	h := BearerWorkspace(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		sawWorkspace, ok = WorkspaceID(r.Context())
		if !ok {
			t.Error("expected workspace in context")
		}
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("status %d, want %d", rr.Code, http.StatusTeapot)
	}
	if sawWorkspace != "ws-42" {
		t.Fatalf("workspace %q, want ws-42", sawWorkspace)
	}
}

func TestBearerWorkspace_skipsAuthRoutes(t *testing.T) {
	h := BearerWorkspace("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sign-in", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBearerWorkspace_skipsHealth(t *testing.T) {
	h := BearerWorkspace("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestBearerWorkspace_missingHeader(t *testing.T) {
	h := BearerWorkspace("secret")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next must not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type %q, want application/json; charset=utf-8", ct)
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Message != "Unauthorized" {
		t.Fatalf("message %q", body.Message)
	}
}

func TestBearerWorkspace_invalidSecret(t *testing.T) {
	const secret = "01234567890123456789012345678901"
	tok, err := jwtutil.IssueAccessToken("user-1", "ws-42", secret, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	h := BearerWorkspace("wrongwrongwrongwrongwrongwrongwr")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next must not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/pull", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestParseBearerHeader_caseInsensitiveScheme(t *testing.T) {
	raw, ok := parseBearerHeader("BeAreR abc.def")
	if !ok || raw != "abc.def" {
		t.Fatalf("got %q ok=%v", raw, ok)
	}
}

func TestWorkspaceID_emptyStoredRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	ctx = contextWithWorkspace(ctx, "")
	req = req.WithContext(ctx)

	_, ok := WorkspaceID(req.Context())
	if ok {
		t.Fatal("expected empty workspace to be rejected")
	}
}

func contextWithWorkspace(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, workspaceIDCtxKey{}, id)
}
