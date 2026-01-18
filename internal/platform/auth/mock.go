package auth

import (
	"context"
)

// MockVerifier provides fake token verification for tests.
type MockVerifier struct {
	User  *FirebaseUser
	Error error
}

// Verify returns the configured user or error.
func (m *MockVerifier) Verify(_ context.Context, _ string) (*FirebaseUser, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.User, nil
}

// TestUser returns a standard test user.
func TestUser() *FirebaseUser {
	return &FirebaseUser{
		UID:           "test-user-123",
		Email:         "test@example.com",
		EmailVerified: true,
	}
}

// Compile-time interface check
var _ Verifier = (*MockVerifier)(nil)
