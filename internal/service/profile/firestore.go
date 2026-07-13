package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/janisto/huma-playground/internal/platform/audit"
)

const profilesCollection = "profiles"

// categorizeError converts errors to audit-safe categories.
func categorizeError(err error) string {
	switch {
	case errors.Is(err, ErrAlreadyExists):
		return "already_exists"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	default:
		return "internal_error"
	}
}

func classifyDependencyError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(ErrUnavailable, err)
	}
	switch status.Code(err) {
	case codes.Aborted, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Unavailable:
		return errors.Join(ErrUnavailable, err)
	default:
		return err
	}
}

// firestoreProfile maps to Firestore document structure.
type firestoreProfile struct {
	FirstName    string    `firestore:"first_name"`
	LastName     string    `firestore:"last_name"`
	ContactEmail string    `firestore:"contact_email"`
	PhoneNumber  string    `firestore:"phone_number"`
	Marketing    bool      `firestore:"marketing"`
	CreatedAt    time.Time `firestore:"created_at"`
	UpdatedAt    time.Time `firestore:"updated_at"`
}

func toProfile(userID string, fp firestoreProfile) *Profile {
	return &Profile{
		ID:           userID,
		FirstName:    fp.FirstName,
		LastName:     fp.LastName,
		ContactEmail: fp.ContactEmail,
		PhoneNumber:  fp.PhoneNumber,
		Marketing:    fp.Marketing,
		CreatedAt:    fp.CreatedAt,
		UpdatedAt:    fp.UpdatedAt,
	}
}

// FirestoreStore implements Store using Firestore.
type FirestoreStore struct {
	client *firestore.Client
}

// NewFirestoreStore creates a new Firestore-backed store.
func NewFirestoreStore(client *firestore.Client) *FirestoreStore {
	return &FirestoreStore{client: client}
}

// Create atomically creates a profile if it does not already exist.
func (s *FirestoreStore) Create(ctx context.Context, userID string, params CreateParams) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(userID)
	now := time.Now().UTC()
	fp := firestoreProfile{
		FirstName:    params.FirstName,
		LastName:     params.LastName,
		ContactEmail: params.ContactEmail,
		PhoneNumber:  params.PhoneNumber,
		Marketing:    params.Marketing,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := docRef.Create(ctx, fp)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			err = ErrAlreadyExists
		} else {
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "create", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrAlreadyExists) {
			return nil, err
		}
		return nil, fmt.Errorf("create profile: %w", err)
	}

	audit.LogEvent(ctx, "create", userID, "profile", userID, "success", nil)

	return toProfile(userID, fp), nil
}

// Get retrieves a profile by user ID.
func (s *FirestoreStore) Get(ctx context.Context, userID string) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(userID)
	doc, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		err = classifyDependencyError(err)
		return nil, fmt.Errorf("get profile: %w", err)
	}

	var fp firestoreProfile
	if err := doc.DataTo(&fp); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}

	return toProfile(userID, fp), nil
}

// Update updates a profile using a transaction for atomicity.
func (s *FirestoreStore) Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(userID)

	var result *Profile

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrNotFound
			}
			return err
		}

		var fp firestoreProfile
		if err := doc.DataTo(&fp); err != nil {
			return err
		}

		updates := make([]firestore.Update, 0, 6)
		if params.FirstName != nil {
			fp.FirstName = *params.FirstName
			updates = append(updates, firestore.Update{Path: "first_name", Value: fp.FirstName})
		}
		if params.LastName != nil {
			fp.LastName = *params.LastName
			updates = append(updates, firestore.Update{Path: "last_name", Value: fp.LastName})
		}
		if params.ContactEmail != nil {
			fp.ContactEmail = *params.ContactEmail
			updates = append(updates, firestore.Update{Path: "contact_email", Value: fp.ContactEmail})
		}
		if params.PhoneNumber != nil {
			fp.PhoneNumber = *params.PhoneNumber
			updates = append(updates, firestore.Update{Path: "phone_number", Value: fp.PhoneNumber})
		}
		if params.Marketing != nil {
			fp.Marketing = *params.Marketing
			updates = append(updates, firestore.Update{Path: "marketing", Value: fp.Marketing})
		}
		fp.UpdatedAt = time.Now().UTC()
		updates = append(updates, firestore.Update{Path: "updated_at", Value: fp.UpdatedAt})

		if err := tx.Update(docRef, updates); err != nil {
			return err
		}

		result = toProfile(userID, fp)
		return nil
	})
	if err != nil {
		err = classifyDependencyError(err)
		audit.LogEvent(ctx, "update", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}

	audit.LogEvent(ctx, "update", userID, "profile", userID, "success", nil)

	return result, nil
}

// Delete atomically removes an existing profile.
func (s *FirestoreStore) Delete(ctx context.Context, userID string) error {
	docRef := s.client.Collection(profilesCollection).Doc(userID)
	_, err := docRef.Delete(ctx, firestore.Exists)
	if err != nil {
		switch status.Code(err) {
		case codes.FailedPrecondition, codes.NotFound:
			err = ErrNotFound
		default:
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "delete", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrNotFound) {
			return err
		}
		return fmt.Errorf("delete profile: %w", err)
	}

	audit.LogEvent(ctx, "delete", userID, "profile", userID, "success", nil)

	return nil
}

// Compile-time interface check
var _ Store = (*FirestoreStore)(nil)
