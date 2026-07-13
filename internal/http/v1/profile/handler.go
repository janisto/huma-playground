package profile

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	obs "github.com/janisto/huma-observability"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/timeutil"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Register registers profile endpoints.
func Register(api huma.API, prefix string, store profilesvc.Store) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-profile",
		Method:        http.MethodPost,
		Path:          "/profile",
		Summary:       "Create user profile",
		Description:   "Creates a profile for the authenticated user.",
		Tags:          []string{"Profile"},
		DefaultStatus: http.StatusCreated,
		Security:      auth.RequireAuth(),
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusConflict,
			http.StatusRequestTimeout,
			http.StatusRequestEntityTooLarge,
			http.StatusUnsupportedMediaType,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *ProfileCreateInput) (*ProfileCreateOutput, error) {
		user := auth.UserFromContext(ctx)

		profile, err := store.Create(ctx, user.UID, profilesvc.CreateParams{
			FirstName:    input.Body.FirstName,
			LastName:     input.Body.LastName,
			ContactEmail: input.Body.ContactEmail,
			PhoneNumber:  input.Body.PhoneNumber,
			Marketing:    input.Body.Marketing,
		})
		if err != nil {
			return nil, mapServiceError(ctx, "create", err)
		}
		return &ProfileCreateOutput{
			Location: prefix + "/profile",
			Body:     toHTTPProfile(profile),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-profile",
		Method:      http.MethodGet,
		Path:        "/profile",
		Summary:     "Get current user's profile",
		Description: "Retrieves the profile for the authenticated user.",
		Tags:        []string{"Profile"},
		Security:    auth.RequireAuth(),
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, _ *ProfileGetInput) (*ProfileGetOutput, error) {
		user := auth.UserFromContext(ctx)

		profile, err := store.Get(ctx, user.UID)
		if err != nil {
			return nil, mapServiceError(ctx, "get", err)
		}
		return &ProfileGetOutput{
			Body: toHTTPProfile(profile),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-profile",
		Method:      http.MethodPatch,
		Path:        "/profile",
		Summary:     "Update current user's profile",
		Description: "Updates fields on the authenticated user's profile. Only provided fields are updated.",
		Tags:        []string{"Profile"},
		Security:    auth.RequireAuth(),
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusRequestTimeout,
			http.StatusRequestEntityTooLarge,
			http.StatusUnsupportedMediaType,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *ProfileUpdateInput) (*ProfileUpdateOutput, error) {
		user := auth.UserFromContext(ctx)
		if !hasProfileUpdateFields(input) {
			return nil, huma.Error422UnprocessableEntity("at least one field must be provided")
		}

		profile, err := store.Update(ctx, user.UID, profilesvc.UpdateParams{
			FirstName:    input.Body.FirstName,
			LastName:     input.Body.LastName,
			ContactEmail: input.Body.ContactEmail,
			PhoneNumber:  input.Body.PhoneNumber,
			Marketing:    input.Body.Marketing,
		})
		if err != nil {
			return nil, mapServiceError(ctx, "update", err)
		}
		return &ProfileUpdateOutput{
			Body: toHTTPProfile(profile),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-profile",
		Method:        http.MethodDelete,
		Path:          "/profile",
		Summary:       "Delete current user's profile",
		Description:   "Permanently deletes the authenticated user's profile.",
		Tags:          []string{"Profile"},
		DefaultStatus: http.StatusNoContent,
		Security:      auth.RequireAuth(),
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, _ *ProfileDeleteInput) (*struct{}, error) {
		user := auth.UserFromContext(ctx)

		if err := store.Delete(ctx, user.UID); err != nil {
			return nil, mapServiceError(ctx, "delete", err)
		}
		return nil, nil
	})
}

func hasProfileUpdateFields(input *ProfileUpdateInput) bool {
	return input.Body.FirstName != nil ||
		input.Body.LastName != nil ||
		input.Body.ContactEmail != nil ||
		input.Body.PhoneNumber != nil ||
		input.Body.Marketing != nil
}

func mapServiceError(ctx context.Context, operation string, err error) error {
	switch {
	case errors.Is(err, profilesvc.ErrNotFound):
		return huma.Error404NotFound("profile not found")
	case errors.Is(err, profilesvc.ErrAlreadyExists):
		return huma.Error409Conflict("profile already exists")
	case errors.Is(err, profilesvc.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		obs.Logger(ctx).Warn("profile store unavailable",
			zap.String("operation", operation), zap.Error(err))
		return huma.Error503ServiceUnavailable("profile service temporarily unavailable")
	default:
		obs.Logger(ctx).Error("profile operation failed",
			zap.String("operation", operation), zap.Error(err))
		return huma.Error500InternalServerError("internal error")
	}
}

func toHTTPProfile(p *profilesvc.Profile) Profile {
	return Profile{
		ID:           p.ID,
		FirstName:    p.FirstName,
		LastName:     p.LastName,
		ContactEmail: p.ContactEmail,
		PhoneNumber:  p.PhoneNumber,
		Marketing:    p.Marketing,
		CreatedAt:    timeutil.Time{Time: p.CreatedAt},
		UpdatedAt:    timeutil.Time{Time: p.UpdatedAt},
	}
}
