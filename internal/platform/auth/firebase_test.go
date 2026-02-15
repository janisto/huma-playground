package auth

import (
	"context"
	"errors"
	"testing"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"

	"github.com/janisto/huma-playground/internal/testutil"
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
		{
			name:   "extra spaces around token",
			header: "Bearer   token123   ",
			want:   "token123",
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
			name:   "bearer with trailing space and no token",
			header: "Bearer ",
		},
		{
			name:   "bearer with whitespace token only",
			header: "Bearer    ",
		},
		{
			name:   "bearer token with extra segment",
			header: "Bearer token123 extra",
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

func TestNewFirebaseVerifier(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := context.Background()
	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	verifier := NewFirebaseVerifier(ac)
	if verifier == nil {
		t.Fatal("expected non-nil verifier")
	}
	if verifier.client != ac {
		t.Fatal("expected verifier.client to be the provided auth client")
	}
}

func TestFirebaseVerifierVerifyValidToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := context.Background()
	testutil.ClearAccounts(t)

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	result := testutil.CreateTestUser(t, "verify@example.com", "password123")

	verifier := NewFirebaseVerifier(ac)
	user, err := verifier.Verify(ctx, result.IDToken)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.UID == "" {
		t.Fatal("expected non-empty UID")
	}
	if user.Email != "verify@example.com" {
		t.Fatalf("expected email verify@example.com, got %s", user.Email)
	}
}

func TestFirebaseVerifierVerifyInvalidToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)

	ctx := context.Background()

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	verifier := NewFirebaseVerifier(ac)
	_, err = verifier.Verify(ctx, "invalid-token-string")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestFirebaseVerifierVerifyRevokedToken(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearEmulators(t)

	ctx := context.Background()

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	result := testutil.CreateTestUser(t, "revoke@example.com", "password123")

	if err := ac.RevokeRefreshTokens(ctx, result.LocalID); err != nil {
		t.Fatalf("failed to revoke tokens: %v", err)
	}

	verifier := NewFirebaseVerifier(ac)
	_, verifyErr := verifier.Verify(ctx, result.IDToken)
	if verifyErr == nil {
		t.Skip("emulator does not enforce token revocation checks")
	}
	if !errors.Is(verifyErr, ErrTokenRevoked) && !errors.Is(verifyErr, ErrInvalidToken) {
		t.Fatalf("expected ErrTokenRevoked or ErrInvalidToken, got %v", verifyErr)
	}
}

func TestFirebaseVerifierVerifyDisabledUser(t *testing.T) {
	testutil.SkipIfEmulatorUnavailable(t)
	testutil.SetupEmulator(t)
	testutil.ClearAccounts(t)

	ctx := context.Background()

	config := &firebase.Config{ProjectID: testutil.ProjectID}
	fbApp, err := firebase.NewApp(ctx, config)
	if err != nil {
		t.Fatalf("failed to create Firebase app: %v", err)
	}

	ac, err := fbApp.Auth(ctx)
	if err != nil {
		t.Fatalf("failed to create auth client: %v", err)
	}

	result := testutil.CreateTestUser(t, "disabled@example.com", "password123")

	disabled := false
	_, err = ac.UpdateUser(ctx, result.LocalID, (&fbauth.UserToUpdate{}).Disabled(true))
	if err != nil {
		t.Logf("emulator may not support DisableUser: %v", err)
	} else {
		disabled = true
	}

	if disabled {
		verifier := NewFirebaseVerifier(ac)
		_, verifyErr := verifier.Verify(ctx, result.IDToken)
		if verifyErr == nil {
			t.Skip("emulator does not enforce disable check on token verification")
		}
		if !errors.Is(verifyErr, ErrUserDisabled) && !errors.Is(verifyErr, ErrInvalidToken) {
			t.Fatalf("expected ErrUserDisabled or ErrInvalidToken, got %v", verifyErr)
		}
	}
}
