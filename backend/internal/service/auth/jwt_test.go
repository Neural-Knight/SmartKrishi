package auth_test

import (
	"testing"

	authservice "github.com/smartkrishi/backend/internal/service/auth"
)

func TestTokenManagerRoundTrip(t *testing.T) {
	tm, err := authservice.NewTokenManager("test-secret-key-at-least-32-chars-long", "HS256", 60)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	token, err := tm.CreateAccessToken("user@example.com", "email")
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}

	claims, err := tm.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.Subject != "user@example.com" {
		t.Fatalf("subject = %q", claims.Subject)
	}
	if claims.AuthProvider != "email" {
		t.Fatalf("auth_provider = %q", claims.AuthProvider)
	}
}

func TestTokenManagerRejectsInvalidToken(t *testing.T) {
	tm, err := authservice.NewTokenManager("test-secret-key-at-least-32-chars-long", "HS256", 60)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	if _, err := tm.VerifyToken("not-a-valid-token"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
