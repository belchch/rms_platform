package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/jwtutil"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour

	// bcryptDummyHash is a valid bcrypt cost-10 hash; used only when the email is unknown
	// so CompareHashAndPassword still runs and timing is less revealing.
	bcryptDummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
)

type SignInInput struct {
	Body struct {
		Email    string `json:"email" doc:"User email"`
		Password string `json:"password" doc:"User password"`
	}
}

type SignInOutput struct {
	Body struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
}

type RefreshInput struct {
	Body struct {
		RefreshToken string `json:"refreshToken"`
	}
}

type RefreshOutput struct {
	Body struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
}

type handler struct {
	q         *db.Queries
	pool      *pgxpool.Pool
	jwtSecret string
}

func Register(api huma.API, q *db.Queries, pool *pgxpool.Pool, jwtSecret string) {
	h := &handler{q: q, pool: pool, jwtSecret: jwtSecret}

	huma.Register(api, huma.Operation{
		OperationID: "sign-in",
		Method:      "POST",
		Path:        "/api/v1/auth/sign-in",
		Summary:     "Sign in",
		Tags:        []string{"auth"},
	}, h.signIn)

	huma.Register(api, huma.Operation{
		OperationID: "refresh-token",
		Method:      "POST",
		Path:        "/api/v1/auth/refresh",
		Summary:     "Refresh access token",
		Tags:        []string{"auth"},
	}, h.refresh)
}

func (h *handler) signIn(ctx context.Context, input *SignInInput) (*SignInOutput, error) {
	user, err := h.q.GetUserByEmail(ctx, input.Body.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = bcrypt.CompareHashAndPassword([]byte(bcryptDummyHash), []byte(input.Body.Password))
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return nil, fmt.Errorf("auth sign-in get user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Body.Password)); err != nil {
		return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
	}
	ws, err := h.q.GetWorkspaceByOwnerID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return nil, fmt.Errorf("auth sign-in workspace: %w", err)
	}
	rawRefresh, err := randomOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("auth sign-in refresh entropy: %w", err)
	}
	refreshID, err := randomHexID()
	if err != nil {
		return nil, fmt.Errorf("auth sign-in refresh id: %w", err)
	}
	expires := time.Now().Add(refreshTokenTTL)
	err = h.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID:        refreshID,
		UserID:    user.ID,
		TokenHash: hashRefreshToken(rawRefresh),
		ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("auth sign-in store refresh: %w", err)
	}
	access, err := jwtutil.IssueAccessToken(user.ID, ws.ID, h.jwtSecret, accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("auth sign-in issue access: %w", err)
	}
	out := &SignInOutput{}
	out.Body.AccessToken = access
	out.Body.RefreshToken = rawRefresh
	return out, nil
}

func (h *handler) refresh(ctx context.Context, input *RefreshInput) (out *RefreshOutput, err error) {
	if input.Body.RefreshToken == "" {
		return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
	}
	hash := hashRefreshToken(input.Body.RefreshToken)

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth refresh begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) && err == nil {
			err = fmt.Errorf("auth refresh rollback: %w", rbErr)
		}
	}()

	qtx := h.q.WithTx(tx)
	row, err := qtx.GetRefreshTokenByHashForUpdate(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return nil, fmt.Errorf("auth refresh lookup: %w", err)
	}
	if !row.ExpiresAt.Valid || time.Now().After(row.ExpiresAt.Time) {
		return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
	}
	user, err := qtx.GetUserByID(ctx, row.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return nil, fmt.Errorf("auth refresh user: %w", err)
	}
	ws, err := qtx.GetWorkspaceByOwnerID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return nil, fmt.Errorf("auth refresh workspace: %w", err)
	}
	rawRefresh, err := randomOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("auth refresh entropy: %w", err)
	}
	newRefreshID, err := randomHexID()
	if err != nil {
		return nil, fmt.Errorf("auth refresh id: %w", err)
	}
	expires := time.Now().Add(refreshTokenTTL)
	if err := qtx.DeleteRefreshToken(ctx, row.ID); err != nil {
		return nil, fmt.Errorf("auth refresh delete old: %w", err)
	}
	if err := qtx.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID:        newRefreshID,
		UserID:    user.ID,
		TokenHash: hashRefreshToken(rawRefresh),
		ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("auth refresh store new: %w", err)
	}
	access, err := jwtutil.IssueAccessToken(user.ID, ws.ID, h.jwtSecret, accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("auth refresh issue access: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("auth refresh commit: %w", err)
	}
	out = &RefreshOutput{}
	out.Body.AccessToken = access
	out.Body.RefreshToken = rawRefresh
	return out, nil
}

func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomHexID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func randomOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
