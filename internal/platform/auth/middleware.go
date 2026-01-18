package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
)

// userContextKey is the context key for the authenticated user.
type userContextKey struct{}

// NewAuthMiddleware creates Huma middleware for Firebase authentication.
// It checks the operation's Security requirements and validates tokens.
func NewAuthMiddleware(api huma.API, verifier Verifier) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if len(ctx.Operation().Security) == 0 {
			next(ctx)
			return
		}

		token, err := ExtractBearerToken(ctx.Header("Authorization"))
		if err != nil {
			applog.LogWarn(ctx.Context(), "auth failed: missing or invalid header",
				zap.String("reason", "no_token"))
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		user, err := verifier.Verify(ctx.Context(), token)
		if err != nil {
			reason := categorizeAuthError(err)
			applog.LogWarn(ctx.Context(), "auth failed: token verification failed",
				zap.String("reason", reason))

			if errors.Is(err, ErrCertificateFetch) {
				ctx.SetHeader("Retry-After", "30")
				_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable,
					"authentication service temporarily unavailable")
				return
			}
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx = huma.WithValue(ctx, userContextKey{}, user)
		next(ctx)
	}
}

// categorizeAuthError returns a safe category string for logging.
func categorizeAuthError(err error) string {
	switch {
	case errors.Is(err, ErrTokenExpired):
		return "token_expired"
	case errors.Is(err, ErrTokenRevoked):
		return "token_revoked"
	case errors.Is(err, ErrUserDisabled):
		return "user_disabled"
	case errors.Is(err, ErrCertificateFetch):
		return "certificate_fetch_failed"
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token"
	default:
		return "unknown"
	}
}

// UserFromContext retrieves the authenticated user from context.
// Returns nil if no user is authenticated.
func UserFromContext(ctx context.Context) *FirebaseUser {
	user, _ := ctx.Value(userContextKey{}).(*FirebaseUser)
	return user
}
