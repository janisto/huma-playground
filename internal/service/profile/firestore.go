package profile

import (
	"context"
	"errors"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
)

const profilesCollection = "profiles"

// categorizeError converts errors to audit-safe categories.
func categorizeError(err error) string {
	switch {
	case errors.Is(err, ErrAlreadyExists):
		return "already_exists"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	default:
		return "internal_error"
	}
}

// firestoreProfile maps to Firestore document structure.
type firestoreProfile struct {
	Firstname   string    `firestore:"firstname"`
	Lastname    string    `firestore:"lastname"`
	Email       string    `firestore:"email"`
	PhoneNumber string    `firestore:"phone_number"`
	Marketing   bool      `firestore:"marketing"`
	Terms       bool      `firestore:"terms"`
	CreatedAt   time.Time `firestore:"created_at"`
	UpdatedAt   time.Time `firestore:"updated_at"`
}

// FirestoreStore implements Service using Firestore with transactions.
type FirestoreStore struct {
	client *firestore.Client
}

// NewFirestoreStore creates a new Firestore-backed store.
func NewFirestoreStore(client *firestore.Client) *FirestoreStore {
	return &FirestoreStore{client: client}
}

// Create creates a new profile using a transaction to prevent duplicates.
func (s *FirestoreStore) Create(ctx context.Context, userID string, params CreateParams) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(userID)
	now := time.Now().UTC()

	var result *Profile

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err == nil && doc.Exists() {
			return ErrAlreadyExists
		}
		if err != nil && status.Code(err) != codes.NotFound {
			return err
		}

		fp := firestoreProfile{
			Firstname:   params.Firstname,
			Lastname:    params.Lastname,
			Email:       strings.ToLower(strings.TrimSpace(params.Email)),
			PhoneNumber: strings.TrimSpace(params.PhoneNumber),
			Marketing:   params.Marketing,
			Terms:       params.Terms,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := tx.Set(docRef, fp); err != nil {
			return err
		}

		result = &Profile{
			ID:          userID,
			Firstname:   fp.Firstname,
			Lastname:    fp.Lastname,
			Email:       fp.Email,
			PhoneNumber: fp.PhoneNumber,
			Marketing:   fp.Marketing,
			Terms:       fp.Terms,
			CreatedAt:   fp.CreatedAt,
			UpdatedAt:   fp.UpdatedAt,
		}
		return nil
	})
	if err != nil {
		applog.LogAuditEvent(ctx, "create", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		return nil, err
	}

	applog.LogAuditEvent(ctx, "create", userID, "profile", userID, "success", nil)

	return result, nil
}

// Get retrieves a profile by user ID.
func (s *FirestoreStore) Get(ctx context.Context, userID string) (*Profile, error) {
	docRef := s.client.Collection(profilesCollection).Doc(userID)
	doc, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var fp firestoreProfile
	if err := doc.DataTo(&fp); err != nil {
		return nil, err
	}

	return &Profile{
		ID:          userID,
		Firstname:   fp.Firstname,
		Lastname:    fp.Lastname,
		Email:       fp.Email,
		PhoneNumber: fp.PhoneNumber,
		Marketing:   fp.Marketing,
		Terms:       fp.Terms,
		CreatedAt:   fp.CreatedAt,
		UpdatedAt:   fp.UpdatedAt,
	}, nil
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

		if params.Firstname != nil {
			fp.Firstname = *params.Firstname
		}
		if params.Lastname != nil {
			fp.Lastname = *params.Lastname
		}
		if params.Email != nil {
			fp.Email = strings.ToLower(strings.TrimSpace(*params.Email))
		}
		if params.PhoneNumber != nil {
			fp.PhoneNumber = strings.TrimSpace(*params.PhoneNumber)
		}
		if params.Marketing != nil {
			fp.Marketing = *params.Marketing
		}
		fp.UpdatedAt = time.Now().UTC()

		if err := tx.Set(docRef, fp); err != nil {
			return err
		}

		result = &Profile{
			ID:          userID,
			Firstname:   fp.Firstname,
			Lastname:    fp.Lastname,
			Email:       fp.Email,
			PhoneNumber: fp.PhoneNumber,
			Marketing:   fp.Marketing,
			Terms:       fp.Terms,
			CreatedAt:   fp.CreatedAt,
			UpdatedAt:   fp.UpdatedAt,
		}
		return nil
	})
	if err != nil {
		applog.LogAuditEvent(ctx, "update", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		return nil, err
	}

	applog.LogAuditEvent(ctx, "update", userID, "profile", userID, "success", nil)

	return result, nil
}

// Delete removes a profile using a transaction to ensure it exists.
func (s *FirestoreStore) Delete(ctx context.Context, userID string) error {
	docRef := s.client.Collection(profilesCollection).Doc(userID)

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrNotFound
			}
			return err
		}

		return tx.Delete(docRef)
	})
	if err != nil {
		applog.LogAuditEvent(ctx, "delete", userID, "profile", userID, "failure",
			map[string]any{"error": categorizeError(err)})
		return err
	}

	applog.LogAuditEvent(ctx, "delete", userID, "profile", userID, "success", nil)

	return nil
}

// Compile-time interface check
var _ Service = (*FirestoreStore)(nil)
