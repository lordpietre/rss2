package auth

import (
	"testing"
	"time"
)

func TestGenerateAndValidateToken(t *testing.T) {
	SetJWTSecret("test-secret-key-12345")

	token, err := GenerateToken(1, "test@example.com", "testuser", true)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("expected userID 1, got %d", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", claims.Email)
	}
	if claims.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", claims.Username)
	}
	if !claims.IsAdmin {
		t.Error("expected isAdmin true")
	}
}

func TestValidateInvalidToken(t *testing.T) {
	SetJWTSecret("test-secret-key-12345")

	_, err := ValidateToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestValidateTokenWithWrongSecret(t *testing.T) {
	SetJWTSecret("secret-1")

	token, err := GenerateToken(1, "test@example.com", "testuser", false)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	SetJWTSecret("secret-2")

	_, err = ValidateToken(token)
	if err == nil {
		t.Error("expected error for token with different secret")
	}
}

func TestClaimsHaveExpiration(t *testing.T) {
	SetJWTSecret("test-secret-key-12345")

	token, err := GenerateToken(1, "test@example.com", "testuser", false)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.ExpiresAt == nil {
		t.Error("expected expiration time to be set")
	}

	expTime := claims.ExpiresAt.Time
	expectedExpiration := time.Now().Add(24 * time.Hour)

	if expTime.Sub(expectedExpiration) > time.Minute {
		t.Errorf("expiration time should be ~24 hours from now, got %v", expTime)
	}
}

func TestEmptyToken(t *testing.T) {
	SetJWTSecret("test-secret-key-12345")

	_, err := ValidateToken("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}
