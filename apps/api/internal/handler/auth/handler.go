package auth

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/belchch/rms_platform/api/internal/db"
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

func Register(api huma.API, q *db.Queries, pool *pgxpool.Pool) {
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

func signIn(_ context.Context, input *SignInInput) (*SignInOutput, error) {
	output := &SignInOutput{}
	output.Body.AccessToken = "todo"
	output.Body.RefreshToken = "todo"
	return output, nil
}

func refresh(_ context.Context, input *RefreshInput) (*RefreshOutput, error) {
	output := &RefreshOutput{}
	output.Body.AccessToken = "todo"
	output.Body.RefreshToken = "todo"
	return output, nil
}
