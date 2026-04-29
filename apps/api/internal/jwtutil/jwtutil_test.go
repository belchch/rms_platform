package jwtutil

import (
	"strings"
	"testing"
	"time"
)

func TestIssueAndParseRoundTrip(t *testing.T) {
	t.Parallel()
	secret := strings.Repeat("s", 32)
	tok, err := IssueAccessToken("user-1", "ws-9", secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccessToken(tok, secret)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("subject: got %q", claims.Subject)
	}
	if claims.WorkspaceID != "ws-9" {
		t.Fatalf("workspace: got %q", claims.WorkspaceID)
	}
}

func TestParseWrongSecret(t *testing.T) {
	t.Parallel()
	secret := strings.Repeat("a", 32)
	tok, err := IssueAccessToken("u", "w", secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseAccessToken(tok, strings.Repeat("b", 32))
	if err == nil {
		t.Fatal("expected error")
	}
}
