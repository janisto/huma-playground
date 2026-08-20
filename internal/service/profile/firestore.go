package profile

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/janisto/huma-playground/internal/platform/audit"
	"github.com/janisto/huma-playground/internal/platform/portable"
)

const (
	profilesCollection        = "profiles"
	encodedProfilesDocument   = "_encoded"
	encodedProfilesCollection = "by-uid"
)

func profileDocumentPath(userID string) string {
	if isFirestoreDocumentID(userID) {
		return profilesCollection + "/" + userID
	}
	return profilesCollection + "/" + encodedProfilesDocument + "/" +
		encodedProfilesCollection + "/" + base64.RawURLEncoding.EncodeToString([]byte(userID))
}

func isFirestoreDocumentID(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		!strings.Contains(value, "/") &&
		(!strings.HasPrefix(value, "__") || !strings.HasSuffix(value, "__"))
}

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
	FirstName      string    `firestore:"first_name"`
	LastName       string    `firestore:"last_name"`
	ContactEmail   string    `firestore:"contact_email"`
	PhoneNumber    string    `firestore:"phone_number"`
	MarketingOptIn bool      `firestore:"marketing_opt_in"`
	TermsAccepted  bool      `firestore:"terms_accepted"`
	CreatedAt      time.Time `firestore:"created_at"`
	UpdatedAt      time.Time `firestore:"updated_at"`
}

func toProfile(userID string, fp firestoreProfile) (*Profile, error) {
	createdAt := fp.CreatedAt.UTC()
	updatedAt := fp.UpdatedAt.UTC()
	canonicalEmail, emailOK := portable.NormalizeContactEmail(fp.ContactEmail)
	canonicalPhone, phoneOK := portable.NormalizePhoneNumber(fp.PhoneNumber)
	if !validProfileID(userID) || !portable.ValidBoundedName(fp.FirstName) ||
		!portable.ValidBoundedName(fp.LastName) || !emailOK || canonicalEmail != fp.ContactEmail ||
		!phoneOK || canonicalPhone != fp.PhoneNumber || !fp.TermsAccepted ||
		!validStoredTimestamp(createdAt) || !validStoredTimestamp(updatedAt) || updatedAt.Before(createdAt) {
		return nil, ErrInvalidStored
	}
	return &Profile{
		ID:             userID,
		FirstName:      fp.FirstName,
		LastName:       fp.LastName,
		ContactEmail:   fp.ContactEmail,
		PhoneNumber:    fp.PhoneNumber,
		MarketingOptIn: fp.MarketingOptIn,
		TermsAccepted:  fp.TermsAccepted,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func validProfileID(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 128
}

func validStoredTimestamp(value time.Time) bool {
	return !value.IsZero() && value.Year() >= 0 && value.Year() <= 9999 &&
		value.Nanosecond()%int(time.Millisecond) == 0
}

func validCreateParams(params CreateParams) bool {
	canonicalEmail, emailOK := portable.NormalizeContactEmail(params.ContactEmail)
	canonicalPhone, phoneOK := portable.NormalizePhoneNumber(params.PhoneNumber)
	return portable.ValidBoundedName(params.FirstName) && portable.ValidBoundedName(params.LastName) &&
		emailOK && canonicalEmail == params.ContactEmail && phoneOK && canonicalPhone == params.PhoneNumber &&
		params.TermsAccepted
}

func validUpdateParams(params UpdateParams) bool {
	if params.FirstName != nil && !portable.ValidBoundedName(*params.FirstName) ||
		params.LastName != nil && !portable.ValidBoundedName(*params.LastName) {
		return false
	}
	if params.ContactEmail != nil {
		canonical, ok := portable.NormalizeContactEmail(*params.ContactEmail)
		if !ok || canonical != *params.ContactEmail {
			return false
		}
	}
	if params.PhoneNumber != nil {
		canonical, ok := portable.NormalizePhoneNumber(*params.PhoneNumber)
		if !ok || canonical != *params.PhoneNumber {
			return false
		}
	}
	return params.FirstName != nil || params.LastName != nil || params.ContactEmail != nil ||
		params.PhoneNumber != nil || params.MarketingOptIn != nil
}

// FirestoreStore implements Store using Firestore.
type FirestoreStore struct {
	client *firestore.Client
	now    func() time.Time
}

// NewFirestoreStore creates a new Firestore-backed store.
func NewFirestoreStore(client *firestore.Client) *FirestoreStore {
	return NewFirestoreStoreWithClock(client, time.Now)
}

// NewFirestoreStoreWithClock supplies the application clock used by profile
// lifecycle rules.
func NewFirestoreStoreWithClock(client *firestore.Client, now func() time.Time) *FirestoreStore {
	if now == nil {
		now = time.Now
	}
	return &FirestoreStore{client: client, now: now}
}

func (s *FirestoreStore) timestamp() time.Time {
	return s.now().UTC().Truncate(time.Millisecond)
}

// Create atomically creates a profile if it does not already exist.
func (s *FirestoreStore) Create(ctx context.Context, userID string, params CreateParams) (*Profile, error) {
	if !validProfileID(userID) || !validCreateParams(params) {
		return nil, ErrInvalidStored
	}
	docRef := s.client.Doc(profileDocumentPath(userID))
	now := s.timestamp()
	fp := firestoreProfile{
		FirstName:      params.FirstName,
		LastName:       params.LastName,
		ContactEmail:   params.ContactEmail,
		PhoneNumber:    params.PhoneNumber,
		MarketingOptIn: params.MarketingOptIn,
		TermsAccepted:  params.TermsAccepted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_, err := docRef.Create(ctx, fp)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			err = ErrAlreadyExists
		} else {
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "create", "", "profile", "", "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrAlreadyExists) {
			return nil, err
		}
		return nil, fmt.Errorf("create profile: %w", err)
	}

	audit.LogEvent(ctx, "create", "", "profile", "", "success", nil)

	return toProfile(userID, fp)
}

// Get retrieves a profile by user ID.
func (s *FirestoreStore) Get(ctx context.Context, userID string) (*Profile, error) {
	if !validProfileID(userID) {
		return nil, ErrInvalidStored
	}
	docRef := s.client.Doc(profileDocumentPath(userID))
	doc, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		err = classifyDependencyError(err)
		return nil, fmt.Errorf("get profile: %w", err)
	}

	var fp firestoreProfile
	if decodeErr := doc.DataTo(&fp); decodeErr != nil {
		return nil, fmt.Errorf("decode profile: %w", decodeErr)
	}

	profile, err := toProfile(userID, fp)
	if err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	return profile, nil
}

// Update updates a profile using a transaction for atomicity.
func (s *FirestoreStore) Update(ctx context.Context, userID string, params UpdateParams) (*Profile, error) {
	if !validProfileID(userID) || !validUpdateParams(params) {
		return nil, ErrInvalidStored
	}
	docRef := s.client.Doc(profileDocumentPath(userID))
	now := s.timestamp()

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
		if _, err := toProfile(userID, fp); err != nil {
			return err
		}

		updates := make([]firestore.Update, 0, 6)
		if params.FirstName != nil && *params.FirstName != fp.FirstName {
			fp.FirstName = *params.FirstName
			updates = append(updates, firestore.Update{Path: "first_name", Value: fp.FirstName})
		}
		if params.LastName != nil && *params.LastName != fp.LastName {
			fp.LastName = *params.LastName
			updates = append(updates, firestore.Update{Path: "last_name", Value: fp.LastName})
		}
		if params.ContactEmail != nil && *params.ContactEmail != fp.ContactEmail {
			fp.ContactEmail = *params.ContactEmail
			updates = append(updates, firestore.Update{Path: "contact_email", Value: fp.ContactEmail})
		}
		if params.PhoneNumber != nil && *params.PhoneNumber != fp.PhoneNumber {
			fp.PhoneNumber = *params.PhoneNumber
			updates = append(updates, firestore.Update{Path: "phone_number", Value: fp.PhoneNumber})
		}
		if params.MarketingOptIn != nil && *params.MarketingOptIn != fp.MarketingOptIn {
			fp.MarketingOptIn = *params.MarketingOptIn
			updates = append(updates, firestore.Update{Path: "marketing_opt_in", Value: fp.MarketingOptIn})
		}
		if len(updates) == 0 {
			var err error
			result, err = toProfile(userID, fp)
			return err
		}
		if !now.After(fp.UpdatedAt) {
			if fp.UpdatedAt.Equal(time.Date(9999, 12, 31, 23, 59, 59, 999_000_000, time.UTC)) {
				return ErrTimeOverflow
			}
			fp.UpdatedAt = fp.UpdatedAt.Add(time.Millisecond)
		} else {
			fp.UpdatedAt = now
		}
		updates = append(updates, firestore.Update{Path: "updated_at", Value: fp.UpdatedAt})

		if err := tx.Update(docRef, updates); err != nil {
			return err
		}

		var conversionErr error
		result, conversionErr = toProfile(userID, fp)
		return conversionErr
	})
	if err != nil {
		err = classifyDependencyError(err)
		audit.LogEvent(ctx, "update", "", "profile", "", "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}

	audit.LogEvent(ctx, "update", "", "profile", "", "success", nil)

	return result, nil
}

// Delete atomically removes an existing profile.
func (s *FirestoreStore) Delete(ctx context.Context, userID string) error {
	if !validProfileID(userID) {
		return ErrInvalidStored
	}
	docRef := s.client.Doc(profileDocumentPath(userID))
	_, err := docRef.Delete(ctx, firestore.Exists)
	if err != nil {
		switch status.Code(err) {
		case codes.FailedPrecondition, codes.NotFound:
			err = ErrNotFound
		default:
			err = classifyDependencyError(err)
		}
		audit.LogEvent(ctx, "delete", "", "profile", "", "failure",
			map[string]any{"error": categorizeError(err)})
		if errors.Is(err, ErrNotFound) {
			return err
		}
		return fmt.Errorf("delete profile: %w", err)
	}

	audit.LogEvent(ctx, "delete", "", "profile", "", "success", nil)

	return nil
}

// Compile-time interface check
var _ Store = (*FirestoreStore)(nil)
