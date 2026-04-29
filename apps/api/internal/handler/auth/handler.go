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
	"golang.org/x/crypto/bcrypt"

	"github.com/belchch/rms_platform/api/internal/db"
	"github.com/belchch/rms_platform/api/internal/jwtutil"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
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

func Register(api huma.API, q *db.Queries, jwtSecret string) {
	signIn := func(ctx context.Context, input *SignInInput) (*SignInOutput, error) {
		user, err := q.GetUserByEmail(ctx, input.Body.Email)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
			}
			return nil, fmt.Errorf("auth sign-in get user: %w", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Body.Password)); err != nil {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		ws, err := q.GetWorkspaceByOwnerID(ctx, user.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.NewError(http.StatusInternalServerError, "internal error")
			}
			return nil, fmt.Errorf("auth sign-in workspace: %w", err)
		}
		access, err := jwtutil.IssueAccessToken(user.ID, ws.ID, jwtSecret, accessTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("auth sign-in issue access: %w", err)
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
		err = q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
			ID:        refreshID,
			UserID:    user.ID,
			TokenHash: hashRefreshToken(rawRefresh),
			ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("auth sign-in store refresh: %w", err)
		}
		out := &SignInOutput{}
		out.Body.AccessToken = access
		out.Body.RefreshToken = rawRefresh
		return out, nil
	}

	refresh := func(ctx context.Context, input *RefreshInput) (*RefreshOutput, error) {
		if input.Body.RefreshToken == "" {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		hash := hashRefreshToken(input.Body.RefreshToken)
		row, err := q.GetRefreshTokenByHash(ctx, hash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
			}
			return nil, fmt.Errorf("auth refresh lookup: %w", err)
		}
		if !row.ExpiresAt.Valid || time.Now().After(row.ExpiresAt.Time) {
			return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		user, err := q.GetUserByID(ctx, row.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.NewError(http.StatusUnauthorized, "invalid credentials")
			}
			return nil, fmt.Errorf("auth refresh user: %w", err)
		}
		ws, err := q.GetWorkspaceByOwnerID(ctx, user.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, huma.NewError(http.StatusInternalServerError, "internal error")
			}
			return nil, fmt.Errorf("auth refresh workspace: %w", err)
		}
		access, err := jwtutil.IssueAccessToken(user.ID, ws.ID, jwtSecret, accessTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("auth refresh issue access: %w", err)
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
		if err := q.DeleteRefreshToken(ctx, row.ID); err != nil {
			return nil, fmt.Errorf("auth refresh delete old: %w", err)
		}
		err = q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
			ID:        newRefreshID,
			UserID:    user.ID,
			TokenHash: hashRefreshToken(rawRefresh),
			ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("auth refresh store new: %w", err)
		}
		out := &RefreshOutput{}
		out.Body.AccessToken = access
		out.Body.RefreshToken = rawRefresh
		return out, nil
	}

	huma.Register(api, huma.Operation{
		OperationID: "sign-in",
		Method:      "POST",
		Path:        "/api/v1/auth/sign-in",
		Summary:     "Sign in",
		Tags:        []string{"auth"},
	}, signIn)

	huma.Register(api, huma.Operation{
		OperationID: "refresh-token",
		Method:      "POST",
		Path:        "/api/v1/auth/refresh",
		Summary:     "Refresh access token",
		Tags:        []string{"auth"},
	}, refresh)
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
