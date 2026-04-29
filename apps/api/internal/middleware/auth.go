package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/belchch/rms_platform/api/internal/jwtutil"
)

type workspaceIDCtxKey struct{}

func WorkspaceID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(workspaceIDCtxKey{}).(string)
	return v, ok && v != ""
}

func BearerWorkspace(api huma.API, secret string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		for _, sec := range ctx.Operation().Security {
			if _, needsBearer := sec["bearerAuth"]; !needsBearer {
				continue
			}
			raw, ok := parseBearerHeader(ctx.Header("Authorization"))
			if !ok {
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
				return
			}
			claims, err := jwtutil.ParseAccessToken(raw, secret)
			if err != nil || claims.WorkspaceID == "" {
				huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
				return
			}
			next(huma.WithValue(ctx, workspaceIDCtxKey{}, claims.WorkspaceID))
			return
		}
		next(ctx)
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
