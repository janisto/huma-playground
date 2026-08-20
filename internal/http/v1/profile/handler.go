package profile

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	obs "github.com/janisto/huma-observability/v2"
	"go.uber.org/zap"

	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/portable"
	"github.com/janisto/huma-playground/internal/platform/timeutil"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Register registers profile endpoints.
func Register(api huma.API, prefix string, store profilesvc.Store) {
	huma.Register(api, huma.Operation{
		OperationID:   "createProfile",
		Method:        http.MethodPost,
		Path:          "/profile",
		Summary:       "Create user profile",
		Description:   "Creates a profile for the authenticated user.",
		Tags:          []string{"Profile"},
		DefaultStatus: http.StatusCreated,
		Security:      auth.RequireAuth(),
		MaxBodyBytes:  portable.MaxRequestBodyBytes + 1,
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusConflict,
			http.StatusNotAcceptable,
			http.StatusRequestEntityTooLarge,
			http.StatusUnsupportedMediaType,
			http.StatusUnprocessableEntity,
			http.StatusInternalServerError,
			http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *ProfileCreateInput) (*ProfileCreateOutput, error) {
		user := auth.UserFromContext(ctx)
		params, validationErr := validateCreate(ctx, input)
		if validationErr != nil {
			return nil, validationErr
		}

		profile, err := store.Create(ctx, user.UID, params)
		if err != nil {
			return nil, mapServiceError(ctx, "create", err)
		}
		return &ProfileCreateOutput{
			Location: prefix + "/profile",
			Body:     toHTTPProfile(profile),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getProfile",
		Method:      http.MethodGet,
		Path:        "/profile",
		Summary:     "Get current user's profile",
		Description: "Retrieves the profile for the authenticated user.",
		Tags:        []string{"Profile"},
		Security:    auth.RequireAuth(),
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusNotAcceptable,
			http.StatusInternalServerError,
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
		OperationID:  "updateProfile",
		Method:       http.MethodPatch,
		Path:         "/profile",
		Summary:      "Update current user's profile",
		Description:  "Updates fields on the authenticated user's profile. Only provided fields are updated.",
		Tags:         []string{"Profile"},
		Security:     auth.RequireAuth(),
		MaxBodyBytes: portable.MaxRequestBodyBytes + 1,
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusNotAcceptable,
			http.StatusRequestEntityTooLarge,
			http.StatusUnsupportedMediaType,
			http.StatusUnprocessableEntity,
			http.StatusInternalServerError,
			http.StatusServiceUnavailable,
		},
	}, func(ctx context.Context, input *ProfileUpdateInput) (*ProfileUpdateOutput, error) {
		user := auth.UserFromContext(ctx)
		params, validationErr := validateUpdate(ctx, input)
		if validationErr != nil {
			return nil, validationErr
		}

		profile, err := store.Update(ctx, user.UID, params)
		if err != nil {
			return nil, mapServiceError(ctx, "update", err)
		}
		return &ProfileUpdateOutput{
			Body: toHTTPProfile(profile),
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteProfile",
		Method:        http.MethodDelete,
		Path:          "/profile",
		Summary:       "Delete current user's profile",
		Description:   "Permanently deletes the authenticated user's profile.",
		Tags:          []string{"Profile"},
		DefaultStatus: http.StatusNoContent,
		Security:      auth.RequireAuth(),
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusInternalServerError,
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
		input.Body.MarketingOptIn != nil
}

func validateCreate(ctx context.Context, input *ProfileCreateInput) (profilesvc.CreateParams, error) {
	if !portable.ValidBoundedName(input.Body.FirstName) || !portable.ValidBoundedName(input.Body.LastName) ||
		!input.Body.TermsAccepted {
		return profilesvc.CreateParams{}, portable.ErrorForContext(ctx, portable.CodeValidationFailed)
	}
	contactEmail, emailOK := portable.NormalizeContactEmail(input.Body.ContactEmail)
	phoneNumber, phoneOK := portable.NormalizePhoneNumber(input.Body.PhoneNumber)
	if !emailOK || !phoneOK {
		return profilesvc.CreateParams{}, portable.ErrorForContext(ctx, portable.CodeValidationFailed)
	}
	marketingOptIn := false
	if input.Body.MarketingOptIn != nil {
		marketingOptIn = *input.Body.MarketingOptIn
	}
	return profilesvc.CreateParams{
		FirstName:      input.Body.FirstName,
		LastName:       input.Body.LastName,
		ContactEmail:   contactEmail,
		PhoneNumber:    phoneNumber,
		MarketingOptIn: marketingOptIn,
		TermsAccepted:  true,
	}, nil
}

func validateUpdate(ctx context.Context, input *ProfileUpdateInput) (profilesvc.UpdateParams, error) {
	if !hasProfileUpdateFields(input) {
		return profilesvc.UpdateParams{}, portable.ErrorForContext(ctx, portable.CodeValidationFailed)
	}
	params := profilesvc.UpdateParams{
		FirstName:      input.Body.FirstName,
		LastName:       input.Body.LastName,
		MarketingOptIn: input.Body.MarketingOptIn,
	}
	if params.FirstName != nil && !portable.ValidBoundedName(*params.FirstName) ||
		params.LastName != nil && !portable.ValidBoundedName(*params.LastName) {
		return profilesvc.UpdateParams{}, portable.ErrorForContext(ctx, portable.CodeValidationFailed)
	}
	if input.Body.ContactEmail != nil {
		normalized, ok := portable.NormalizeContactEmail(*input.Body.ContactEmail)
		if !ok {
			return profilesvc.UpdateParams{}, portable.ErrorForContext(ctx, portable.CodeValidationFailed)
		}
		params.ContactEmail = &normalized
	}
	if input.Body.PhoneNumber != nil {
		normalized, ok := portable.NormalizePhoneNumber(*input.Body.PhoneNumber)
		if !ok {
			return profilesvc.UpdateParams{}, portable.ErrorForContext(ctx, portable.CodeValidationFailed)
		}
		params.PhoneNumber = &normalized
	}
	return params, nil
}

func mapServiceError(ctx context.Context, operation string, err error) error {
	switch {
	case errors.Is(err, profilesvc.ErrNotFound):
		return portable.ErrorForContext(ctx, portable.CodeProfileNotFound)
	case errors.Is(err, profilesvc.ErrAlreadyExists):
		return portable.ErrorForContext(ctx, portable.CodeProfileExists)
	case errors.Is(err, profilesvc.ErrUnavailable),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled):
		obs.Logger(ctx).Warn("profile store unavailable",
			zap.String("operation", operation), zap.String("error_category", "dependency_unavailable"))
		return portable.ErrorForContext(ctx, portable.CodeDependencyUnavailable)
	default:
		obs.Logger(ctx).Error("profile operation failed",
			zap.String("operation", operation), zap.String("error_category", "internal_error"))
		return portable.ErrorForContext(ctx, portable.CodeInternalError)
	}
}

func toHTTPProfile(p *profilesvc.Profile) Profile {
	return Profile{
		ID:             p.ID,
		FirstName:      p.FirstName,
		LastName:       p.LastName,
		ContactEmail:   p.ContactEmail,
		PhoneNumber:    p.PhoneNumber,
		MarketingOptIn: p.MarketingOptIn,
		TermsAccepted:  p.TermsAccepted,
		CreatedAt:      timeutil.Time{Time: p.CreatedAt},
		UpdatedAt:      timeutil.Time{Time: p.UpdatedAt},
	}
}
