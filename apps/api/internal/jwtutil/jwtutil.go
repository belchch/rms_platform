package jwtutil

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	WorkspaceID string `json:"workspaceId"`
	jwt.RegisteredClaims
}

func IssueAccessToken(userID, workspaceID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		WorkspaceID: workspaceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("jwtutil: sign access token: %w", err)
	}
	return signed, nil
}

func ParseAccessToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("jwtutil: unexpected signing method %T", token.Method)
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwtutil: parse access token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwtutil: invalid access token")
	}
	return claims, nil
}
