package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidatorValidatesHS256AndUserID(t *testing.T) {
	validator, err := NewHMACValidator("secret", "beester", "api", "sub", 0)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	claims := jwt.MapClaims{
		"sub": "user-123",
		"iss": "beester",
		"aud": "api",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	userID, err := validator.Validate(signed)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("expected user-123, got %q", userID)
	}
}

func TestValidatorRejectsExpiredToken(t *testing.T) {
	validator, err := NewHMACValidator("secret", "", "", "sub", 0)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(-time.Minute).Unix(),
	})
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := validator.Validate(signed); err == nil {
		t.Fatal("expected expired token error")
	}
}
