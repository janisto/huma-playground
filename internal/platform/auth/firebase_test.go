package auth

import (
	"errors"
	"testing"
)

func TestExtractBearerTokenValid(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "lowercase bearer",
			header: "bearer token123",
			want:   "token123",
		},
		{
			name:   "uppercase Bearer",
			header: "Bearer token123",
			want:   "token123",
		},
		{
			name:   "mixed case BEARER",
			header: "BEARER token123",
			want:   "token123",
		},
		{
			name:   "token with dots (JWT-like)",
			header: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig",
			want:   "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBearerToken(tt.header)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractBearerTokenEmpty(t *testing.T) {
	_, err := ExtractBearerToken("")
	if !errors.Is(err, ErrNoToken) {
		t.Fatalf("expected ErrNoToken, got %v", err)
	}
}

func TestExtractBearerTokenInvalid(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "missing scheme",
			header: "token123",
		},
		{
			name:   "wrong scheme",
			header: "Basic token123",
		},
		{
			name:   "basic auth format",
			header: "Basic dXNlcjpwYXNz",
		},
		{
			name:   "bearer without token",
			header: "Bearer",
		},
		{
			name:   "only spaces",
			header: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractBearerToken(tt.header)
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestFirebaseUserFields(t *testing.T) {
	user := FirebaseUser{
		UID:           "user-123",
		Email:         "test@example.com",
		EmailVerified: true,
	}

	if user.UID != "user-123" {
		t.Fatalf("expected UID user-123, got %s", user.UID)
	}
	if user.Email != "test@example.com" {
		t.Fatalf("expected email test@example.com, got %s", user.Email)
	}
	if !user.EmailVerified {
		t.Fatal("expected EmailVerified to be true")
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrNoToken", ErrNoToken, "missing authorization header"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token"},
		{"ErrTokenExpired", ErrTokenExpired, "token expired"},
		{"ErrTokenRevoked", ErrTokenRevoked, "token revoked"},
		{"ErrUserDisabled", ErrUserDisabled, "user disabled"},
		{"ErrCertificateFetch", ErrCertificateFetch, "failed to fetch certificates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Fatalf("got %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}
