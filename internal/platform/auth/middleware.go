package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/janisto/huma-observability"
	"go.uber.org/zap"
)

// userContextKey is the context key for the authenticated user.
type userContextKey struct{}

// NewAuthMiddleware creates Huma middleware for Firebase authentication.
// It checks the operation's Security requirements and validates tokens.
func NewAuthMiddleware(api huma.API, verifier Verifier) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !requiresBearerAuth(ctx.Operation().Security) {
			next(ctx)
			return
		}

		token, err := ExtractBearerToken(ctx.Header("Authorization"))
		if err != nil {
			obs.Logger(ctx.Context()).Warn("auth failed: missing or invalid header",
				zap.String("reason", "no_token"))
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			writeAuthError(api, ctx, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		if verifier == nil {
			obs.Logger(ctx.Context()).Error("auth failed: verifier is unavailable")
			writeAuthUnavailable(api, ctx)
			return
		}

		user, err := verifier.Verify(ctx.Context(), token)
		if err != nil {
			if errors.Is(err, context.Canceled) && ctx.Context().Err() != nil {
				return
			}
			reason := categorizeAuthError(err)
			obs.Logger(ctx.Context()).Warn("auth failed: token verification failed",
				zap.String("reason", reason))

			if errors.Is(err, context.Canceled) {
				writeAuthUnavailable(api, ctx)
				return
			}
			if isCredentialError(err) {
				ctx.SetHeader("WWW-Authenticate", "Bearer")
				writeAuthError(api, ctx, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			writeAuthUnavailable(api, ctx)
			return
		}
		if user == nil || user.UID == "" {
			obs.Logger(ctx.Context()).Error("auth failed: verifier returned no identity")
			writeAuthUnavailable(api, ctx)
			return
		}

		ctx = huma.WithValue(ctx, userContextKey{}, user)
		next(ctx)
	}
}

func isCredentialError(err error) bool {
	return errors.Is(err, ErrNoToken) ||
		errors.Is(err, ErrInvalidToken) ||
		errors.Is(err, ErrTokenExpired) ||
		errors.Is(err, ErrTokenRevoked) ||
		errors.Is(err, ErrUserDisabled)
}

func writeAuthUnavailable(api huma.API, ctx huma.Context) {
	ctx.SetHeader("Retry-After", "30")
	writeAuthError(api, ctx, http.StatusServiceUnavailable, "authentication service temporarily unavailable")
}

func requiresBearerAuth(requirements []map[string][]string) bool {
	for _, requirement := range requirements {
		if _, ok := requirement[BearerAuthScheme]; ok {
			return true
		}
	}
	return false
}

func writeAuthError(api huma.API, ctx huma.Context, status int, detail string) {
	if err := huma.WriteErr(api, ctx, status, detail); err != nil {
		obs.Logger(ctx.Context()).Error("write authentication error", zap.Error(err))
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
	case errors.Is(err, ErrAuthUnavailable):
		return "dependency_unavailable"
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token"
	case errors.Is(err, context.Canceled):
		return "dependency_unavailable"
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
