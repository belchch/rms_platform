package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/belchch/rms_platform/api/internal/db"
)

const demoUserID = "a0000000-0000-4000-8000-000000000001"

func TestRefreshSecondCallUsesRotatedTokenOnly(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping refresh rotation integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	q := db.New(pool)
	_, err = pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, demoUserID)
	if err != nil {
		t.Fatal(err)
	}

	raw := "cafebabecafebabecafebabecafebabecafebabecafebabecafebabecafebabe"
	rtID := "d0000000-0000-4000-8000-000000000001"
	err = q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID:        rtID,
		UserID:    demoUserID,
		TokenHash: hashRefreshToken(raw),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("test", "0.0.0"))
	Register(api, q, pool, strings.Repeat("j", 32))

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	first := postRefresh(t, ts.URL, raw)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first refresh: status %d", first.StatusCode)
	}
	var firstBody struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(first.Body).Decode(&firstBody); err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	if firstBody.RefreshToken == "" {
		t.Fatal("expected new refresh token in body")
	}

	secondSame := postRefresh(t, ts.URL, raw)
	secondSame.Body.Close()
	if secondSame.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reuse old refresh after rotation: status %d, want 401", secondSame.StatusCode)
	}

	third := postRefresh(t, ts.URL, firstBody.RefreshToken)
	third.Body.Close()
	if third.StatusCode != http.StatusOK {
		t.Fatalf("second valid refresh: status %d", third.StatusCode)
	}
}

func postRefresh(t *testing.T, baseURL, refresh string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"refreshToken": refresh})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/auth/refresh", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
