package profile

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/platform/auth"
	"github.com/janisto/huma-playground/internal/platform/timeutil"
	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

// Register registers profile endpoints.
func Register(api huma.API, svc profilesvc.Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-profile",
		Method:        http.MethodPost,
		Path:          "/profile",
		Summary:       "Create user profile",
		Description:   "Creates a new profile for the authenticated user. Terms must be accepted.",
		Tags:          []string{"Profile"},
		DefaultStatus: http.StatusCreated,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *ProfileCreateInput) (*ProfileCreateOutput, error) {
		user := auth.UserFromContext(ctx)

		if !input.Body.Terms {
			return nil, huma.Error422UnprocessableEntity("terms must be accepted")
		}

		profile, err := svc.Create(ctx, user.UID, profilesvc.CreateParams{
			Firstname:   input.Body.Firstname,
			Lastname:    input.Body.Lastname,
			Email:       input.Body.Email,
			PhoneNumber: input.Body.PhoneNumber,
			Marketing:   input.Body.Marketing,
			Terms:       input.Body.Terms,
		})
		if err != nil {
			return nil, mapServiceError(err)
		}
		return &ProfileCreateOutput{
			Location: "/v1/profile",
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
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, _ *ProfileGetInput) (*ProfileGetOutput, error) {
		user := auth.UserFromContext(ctx)

		profile, err := svc.Get(ctx, user.UID)
		if err != nil {
			return nil, mapServiceError(err)
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
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *ProfileUpdateInput) (*ProfileUpdateOutput, error) {
		user := auth.UserFromContext(ctx)
		if !hasProfileUpdateFields(input) {
			return nil, huma.Error422UnprocessableEntity("at least one field must be provided")
		}

		profile, err := svc.Update(ctx, user.UID, profilesvc.UpdateParams{
			Firstname:   input.Body.Firstname,
			Lastname:    input.Body.Lastname,
			Email:       input.Body.Email,
			PhoneNumber: input.Body.PhoneNumber,
			Marketing:   input.Body.Marketing,
		})
		if err != nil {
			return nil, mapServiceError(err)
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
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, _ *ProfileDeleteInput) (*struct{}, error) {
		user := auth.UserFromContext(ctx)

		if err := svc.Delete(ctx, user.UID); err != nil {
			return nil, mapServiceError(err)
		}
		return nil, nil
	})
}

func hasProfileUpdateFields(input *ProfileUpdateInput) bool {
	return input.Body.Firstname != nil ||
		input.Body.Lastname != nil ||
		input.Body.Email != nil ||
		input.Body.PhoneNumber != nil ||
		input.Body.Marketing != nil
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, profilesvc.ErrNotFound):
		return huma.Error404NotFound("profile not found")
	case errors.Is(err, profilesvc.ErrAlreadyExists):
		return huma.Error409Conflict("profile already exists")
	default:
		return huma.Error500InternalServerError("internal error")
	}
}

func toHTTPProfile(p *profilesvc.Profile) Profile {
	return Profile{
		ID:          p.ID,
		Firstname:   p.Firstname,
		Lastname:    p.Lastname,
		Email:       p.Email,
		PhoneNumber: p.PhoneNumber,
		Marketing:   p.Marketing,
		Terms:       p.Terms,
		CreatedAt:   timeutil.Time{Time: p.CreatedAt},
		UpdatedAt:   timeutil.Time{Time: p.UpdatedAt},
	}
}
