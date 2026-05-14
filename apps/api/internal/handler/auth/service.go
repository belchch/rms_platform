package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/belchch/rms_platform/api/internal/db"
)

type refreshTokenRotation struct {
	user       db.User
	workspace  db.Workspace
	rawRefresh string
}

func rotateRefreshToken(ctx context.Context, q db.Querier, tokenHash string) (refreshTokenRotation, error) {
	row, err := q.GetRefreshTokenByHashForUpdate(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return refreshTokenRotation{}, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return refreshTokenRotation{}, fmt.Errorf("auth refresh lookup: %w", err)
	}
	now := time.Now()
	if !row.ExpiresAt.Valid || now.After(row.ExpiresAt.Time) {
		return refreshTokenRotation{}, huma.NewError(http.StatusUnauthorized, "invalid credentials")
	}
	user, err := q.GetUserByID(ctx, row.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return refreshTokenRotation{}, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return refreshTokenRotation{}, fmt.Errorf("auth refresh user: %w", err)
	}
	ws, err := q.GetWorkspaceByOwnerID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return refreshTokenRotation{}, huma.NewError(http.StatusUnauthorized, "invalid credentials")
		}
		return refreshTokenRotation{}, fmt.Errorf("auth refresh workspace: %w", err)
	}
	rawRefresh, err := randomOpaqueToken()
	if err != nil {
		return refreshTokenRotation{}, fmt.Errorf("auth refresh entropy: %w", err)
	}
	newRefreshID, err := randomHexID()
	if err != nil {
		return refreshTokenRotation{}, fmt.Errorf("auth refresh id: %w", err)
	}
	if err := q.DeleteRefreshToken(ctx, row.ID); err != nil {
		return refreshTokenRotation{}, fmt.Errorf("auth refresh delete old: %w", err)
	}
	if err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID:        newRefreshID,
		UserID:    user.ID,
		TokenHash: hashRefreshToken(rawRefresh),
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(refreshTokenTTL), Valid: true},
	}); err != nil {
		return refreshTokenRotation{}, fmt.Errorf("auth refresh store new: %w", err)
	}
	return refreshTokenRotation{user: user, workspace: ws, rawRefresh: rawRefresh}, nil
}
