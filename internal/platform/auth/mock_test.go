package auth

import (
	"context"
	"errors"
	"testing"
)

func TestMockVerifierReturnsUser(t *testing.T) {
	user := &FirebaseUser{
		UID:           "mock-user-456",
		Email:         "mock@example.com",
		EmailVerified: true,
	}
	verifier := &MockVerifier{User: user}

	got, err := verifier.Verify(context.Background(), "any-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UID != user.UID {
		t.Fatalf("expected UID %s, got %s", user.UID, got.UID)
	}
	if got.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, got.Email)
	}
}

func TestMockVerifierReturnsError(t *testing.T) {
	verifier := &MockVerifier{Error: ErrTokenExpired}

	_, err := verifier.Verify(context.Background(), "expired-token")
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestMockVerifierErrorTakesPrecedence(t *testing.T) {
	user := &FirebaseUser{UID: "user-123"}
	verifier := &MockVerifier{User: user, Error: ErrInvalidToken}

	_, err := verifier.Verify(context.Background(), "token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken when both User and Error are set, got %v", err)
	}
}

func TestTestUserDefaults(t *testing.T) {
	user := TestUser()

	if user.UID != "test-user-123" {
		t.Fatalf("expected UID test-user-123, got %s", user.UID)
	}
	if user.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", user.Email)
	}
	if !user.EmailVerified {
		t.Fatal("expected EmailVerified to be true")
	}
}
