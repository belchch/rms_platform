package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/belchch/rms_platform/api/internal/jwtutil"
)

type workspaceIDCtxKey struct{}

func WorkspaceID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(workspaceIDCtxKey{}).(string)
	return v, ok && v != ""
}

func BearerWorkspace(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bypassBearerAuth(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			raw, ok := parseBearerHeader(r.Header.Get("Authorization"))
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := jwtutil.ParseAccessToken(raw, secret)
			if err != nil || claims.WorkspaceID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), workspaceIDCtxKey{}, claims.WorkspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bypassBearerAuth(path string) bool {
	switch {
	case path == "/health":
		return true
	case strings.HasPrefix(path, "/api/v1/auth/"):
		return true
	default:
		return false
	}
}

func parseBearerHeader(h string) (string, bool) {
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}
