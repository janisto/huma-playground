package auth

import (
	"context"
	"errors"
	"strings"

	fbauth "firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/errorutils"
)

// FirebaseUser represents an authenticated user.
type FirebaseUser struct {
	UID           string
	Email         string
	EmailVerified bool
}

// Error types for authentication failures.
var (
	// ErrNoToken indicates missing Authorization header.
	ErrNoToken = errors.New("missing authorization header")

	// ErrInvalidToken indicates an invalid token format or signature.
	ErrInvalidToken = errors.New("invalid token")

	// ErrTokenExpired indicates the token has expired.
	ErrTokenExpired = errors.New("token expired")

	// ErrTokenRevoked indicates the token has been revoked.
	ErrTokenRevoked = errors.New("token revoked")

	// ErrUserDisabled indicates the user account is disabled.
	ErrUserDisabled = errors.New("user disabled")

	// ErrCertificateFetch indicates a network error fetching public keys.
	// This should result in HTTP 503 (service unavailable).
	ErrCertificateFetch = errors.New("failed to fetch certificates")

	// ErrAuthUnavailable indicates an authentication dependency failure.
	ErrAuthUnavailable = errors.New("authentication service unavailable")
)

// Verifier validates tokens and returns user information.
type Verifier interface {
	Verify(ctx context.Context, token string) (*FirebaseUser, error)
}

// FirebaseVerifier implements Verifier using Firebase Admin SDK.
type FirebaseVerifier struct {
	client *fbauth.Client
}

// NewFirebaseVerifier creates a new verifier with the given auth client.
func NewFirebaseVerifier(client *fbauth.Client) *FirebaseVerifier {
	return &FirebaseVerifier{client: client}
}

// Verify validates a Firebase ID token and checks for revocation.
func (v *FirebaseVerifier) Verify(ctx context.Context, idToken string) (*FirebaseUser, error) {
	if v == nil || v.client == nil {
		return nil, ErrAuthUnavailable
	}
	token, err := v.client.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		switch {
		case fbauth.IsCertificateFetchFailed(err):
			return nil, errors.Join(ErrCertificateFetch, ErrAuthUnavailable, err)
		case fbauth.IsIDTokenExpired(err):
			return nil, errors.Join(ErrTokenExpired, err)
		case fbauth.IsIDTokenRevoked(err):
			return nil, errors.Join(ErrTokenRevoked, err)
		case fbauth.IsUserDisabled(err):
			return nil, errors.Join(ErrUserDisabled, err)
		case fbauth.IsIDTokenInvalid(err):
			return nil, errors.Join(ErrInvalidToken, err)
		case errors.Is(err, context.Canceled), errorutils.IsCancelled(err):
			return nil, errors.Join(context.Canceled, err)
		case errors.Is(err, context.DeadlineExceeded), errorutils.IsDeadlineExceeded(err):
			return nil, errors.Join(ErrAuthUnavailable, context.DeadlineExceeded, err)
		case errorutils.IsUnavailable(err), errorutils.IsInternal(err), errorutils.IsUnknown(err):
			return nil, errors.Join(ErrAuthUnavailable, err)
		default:
			return nil, errors.Join(ErrAuthUnavailable, err)
		}
	}

	email, _ := token.Claims["email"].(string)
	verified, _ := token.Claims["email_verified"].(bool)

	return &FirebaseUser{
		UID:           token.UID,
		Email:         email,
		EmailVerified: verified,
	}, nil
}

// ExtractBearerToken extracts the token from Authorization header.
func ExtractBearerToken(header string) (string, error) {
	if header == "" {
		return "", ErrNoToken
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		return "", ErrInvalidToken
	}
	return parts[1], nil
}

// Compile-time interface check
var _ Verifier = (*FirebaseVerifier)(nil)
