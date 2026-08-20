package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/portable"
)

// identityContextKey is the context key for request-scoped authentication state.
type identityContextKey struct{}

type requestIdentity struct {
	user *FirebaseUser
}

// NewIdentityContextMiddleware installs request-scoped authentication state
// before middleware that observes the Huma context. Authentication can then
// populate the state without replacing the context captured by terminal
// observability middleware.
func NewIdentityContextMiddleware() func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if identityFromContext(ctx.Context()) != nil {
			next(ctx)
			return
		}
		next(huma.WithValue(ctx, identityContextKey{}, &requestIdentity{}))
	}
}

// NewAuthMiddleware creates Huma middleware for Firebase authentication.
// It checks the operation's Security requirements and validates tokens.
func NewAuthMiddleware(api huma.API, verifier Verifier) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !requiresBearerAuth(ctx.Operation().Security) {
			next(ctx)
			return
		}

		authorizationValues := headerValues(ctx, "Authorization")
		var authorization string
		if len(authorizationValues) == 1 && !strings.Contains(authorizationValues[0], ",") {
			authorization = authorizationValues[0]
		}
		token, err := ExtractBearerToken(authorization)
		if err != nil {
			obs.Logger(ctx.Context()).Warn("auth failed: missing or invalid header",
				zap.String("reason", "no_token"))
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			writeAuthError(api, ctx, http.StatusUnauthorized)
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
				writeAuthError(api, ctx, http.StatusUnauthorized)
				return
			}
			writeAuthUnavailable(api, ctx)
			return
		}
		if user == nil || user.UID == "" || !utf8.ValidString(user.UID) || utf8.RuneCountInString(user.UID) > 128 {
			obs.Logger(ctx.Context()).Error("auth failed: verifier returned no identity")
			writeAuthUnavailable(api, ctx)
			return
		}

		identity := identityFromContext(ctx.Context())
		if identity == nil {
			identity = &requestIdentity{}
			ctx = huma.WithValue(ctx, identityContextKey{}, identity)
		}
		identity.user = user
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
	writeAuthError(api, ctx, http.StatusServiceUnavailable)
}

func requiresBearerAuth(requirements []map[string][]string) bool {
	for _, requirement := range requirements {
		if _, ok := requirement[BearerAuthScheme]; ok {
			return true
		}
	}
	return false
}

func writeAuthError(api huma.API, ctx huma.Context, status int) {
	code := portable.CodeUnauthorized
	if status == http.StatusServiceUnavailable {
		code = portable.CodeDependencyUnavailable
	}
	problem := portable.NewProblemForContext(ctx.Context(), code)
	if err := huma.WriteErr(api, ctx, status, problem.Detail); err != nil {
		obs.Logger(ctx.Context()).Error("write authentication error", zap.Error(err))
	}
}

func headerValues(ctx huma.Context, name string) []string {
	values := make([]string, 0, 1)
	ctx.EachHeader(func(candidate, value string) {
		if strings.EqualFold(candidate, name) {
			values = append(values, value)
		}
	})
	return values
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
	identity := identityFromContext(ctx)
	if identity == nil {
		return nil
	}
	return identity.user
}

func identityFromContext(ctx context.Context) *requestIdentity {
	identity, _ := ctx.Value(identityContextKey{}).(*requestIdentity)
	return identity
}
