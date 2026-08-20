package profile

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"

	"github.com/janisto/huma-playground/internal/platform/portable"
)

const ProfileMigrationManifestVersion = 1

type MigrationAuthorization struct {
	TermsAccepted bool   `json:"termsAccepted"`
	Evidence      string `json:"evidence"`
}

type MigrationManifest struct {
	Version int                               `json:"version"`
	Entries map[string]MigrationAuthorization `json:"entries"`
}

type MigrationState string

const (
	MigrationVerified MigrationState = "verified"
	MigrationRequired MigrationState = "migration_required"
	MigrationBlocked  MigrationState = "blocked"
	MigrationApplied  MigrationState = "applied"
)

type MigrationResult struct {
	DocumentFingerprint string
	State               MigrationState
	Reason              string
}

func (manifest MigrationManifest) Validate() error {
	if manifest.Version != ProfileMigrationManifestVersion || manifest.Entries == nil {
		return errors.New("profile migration manifest must have version 1 and an entries object")
	}
	for principal, authorization := range manifest.Entries {
		if !validProfileID(principal) {
			return errors.New("profile migration manifest contains an invalid principal identifier")
		}
		if !authorization.TermsAccepted || !validMigrationEvidence(authorization.Evidence) {
			return errors.New(
				"every profile migration authorization must affirm terms acceptance and cite safe evidence",
			)
		}
	}
	return nil
}

// AuditProfileMigration is read-only and classifies both direct and hardened
// principal-key storage layouts.
func AuditProfileMigration(
	ctx context.Context,
	client *firestore.Client,
	manifest MigrationManifest,
) ([]MigrationResult, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	documents, err := profileMigrationDocuments(ctx, client)
	if err != nil {
		return nil, err
	}
	results := make([]MigrationResult, 0, len(documents)+len(manifest.Entries))
	seen := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		if document.principal == "" {
			results = append(results, MigrationResult{
				DocumentFingerprint: document.fingerprint,
				State:               MigrationBlocked,
				Reason:              "profile document has a noncanonical ownership key",
			})
			continue
		}
		if _, duplicate := seen[document.principal]; duplicate {
			return nil, errors.New("profile migration found duplicate logical principal records")
		}
		seen[document.principal] = struct{}{}
		state, reason, _ := classifyMigration(
			document.snapshot.Data(), document.principal, manifest.Entries[document.principal],
		)
		results = append(results, MigrationResult{
			DocumentFingerprint: document.fingerprint,
			State:               state,
			Reason:              reason,
		})
	}
	for principal := range manifest.Entries {
		if _, exists := seen[principal]; !exists {
			results = append(results, MigrationResult{
				DocumentFingerprint: fingerprintPrincipal(principal),
				State:               MigrationBlocked,
				Reason:              "migration authorization has no matching profile record",
			})
		}
	}
	return results, nil
}

// ApplyProfileMigration performs no writes unless a complete read-only audit
// has no blocked records. Each replacement is then compare-checked in a
// transaction, making the command safe to rerun after an interrupted run.
func ApplyProfileMigration(
	ctx context.Context,
	client *firestore.Client,
	manifest MigrationManifest,
) ([]MigrationResult, error) {
	audit, err := AuditProfileMigration(ctx, client, manifest)
	if err != nil {
		return nil, err
	}
	for _, result := range audit {
		if result.State == MigrationBlocked {
			return audit, errors.New("profile migration is blocked; no records were changed")
		}
	}
	documents, err := profileMigrationDocuments(ctx, client)
	if err != nil {
		return nil, err
	}
	results := make([]MigrationResult, 0, len(documents))
	for _, document := range documents {
		result := MigrationResult{DocumentFingerprint: document.fingerprint}
		transactionErr := client.RunTransaction(
			ctx,
			func(ctx context.Context, transaction *firestore.Transaction) error {
				current, getErr := transaction.Get(document.snapshot.Ref)
				if getErr != nil {
					return getErr
				}
				state, reason, replacement := classifyMigration(
					current.Data(), document.principal, manifest.Entries[document.principal],
				)
				result.State, result.Reason = state, reason
				switch state {
				case MigrationVerified:
					return nil
				case MigrationRequired:
					if setErr := transaction.Set(document.snapshot.Ref, replacement); setErr != nil {
						return setErr
					}
					result.State = MigrationApplied
					return nil
				default:
					return errors.New("profile record became blocked during migration")
				}
			},
		)
		if transactionErr != nil {
			return results, fmt.Errorf("apply profile migration to %s: %w", result.DocumentFingerprint, transactionErr)
		}
		results = append(results, result)
	}
	return results, nil
}

type migrationDocument struct {
	snapshot    *firestore.DocumentSnapshot
	principal   string
	fingerprint string
}

func profileMigrationDocuments(ctx context.Context, client *firestore.Client) ([]migrationDocument, error) {
	if client == nil {
		return nil, errors.New("firestore client is required")
	}
	documents := make([]migrationDocument, 0)
	direct, err := collectMigrationDocuments(ctx, client.Collection(profilesCollection), false)
	if err != nil {
		return nil, err
	}
	documents = append(documents, direct...)
	encoded, err := collectMigrationDocuments(
		ctx,
		client.Collection(profilesCollection).Doc(encodedProfilesDocument).Collection(encodedProfilesCollection),
		true,
	)
	if err != nil {
		return nil, err
	}
	return append(documents, encoded...), nil
}

func collectMigrationDocuments(
	ctx context.Context,
	collection *firestore.CollectionRef,
	encoded bool,
) ([]migrationDocument, error) {
	snapshots, err := collection.Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("iterate profile documents: %w", err)
	}
	result := make([]migrationDocument, 0, len(snapshots))
	for _, snapshot := range snapshots {
		principal := snapshot.Ref.ID
		if encoded {
			decoded, decodeErr := base64.RawURLEncoding.DecodeString(snapshot.Ref.ID)
			if decodeErr != nil || base64.RawURLEncoding.EncodeToString(decoded) != snapshot.Ref.ID {
				principal = ""
			} else {
				principal = string(decoded)
			}
		}
		if !validProfileID(principal) || profileDocumentPath(principal) != snapshot.Ref.Path {
			principal = ""
		}
		fingerprintValue := snapshot.Ref.Path
		if principal != "" {
			fingerprintValue = principal
		}
		result = append(result, migrationDocument{
			snapshot: snapshot, principal: principal, fingerprint: fingerprintPrincipal(fingerprintValue),
		})
	}
	return result, nil
}

func classifyMigration(
	data map[string]any,
	principal string,
	authorization MigrationAuthorization,
) (MigrationState, string, map[string]any) {
	if stored, ok := canonicalProfileData(data, principal); ok {
		return MigrationVerified, "record already satisfies the accepted schema", canonicalProfileMap(stored)
	}
	if !exactLegacyProfileFields(data) {
		return MigrationBlocked, "record is neither an exact canonical record nor the known pre-adoption shape", nil
	}
	if !authorization.TermsAccepted || !validMigrationEvidence(authorization.Evidence) {
		return MigrationBlocked, "legacy record lacks an explicit terms-acceptance evidence entry", nil
	}
	legacy, ok := legacyProfileData(data)
	if !ok {
		return MigrationBlocked, "legacy record contains an invalid field type or lifecycle", nil
	}
	contactEmail, emailOK := portable.NormalizeContactEmail(legacy.ContactEmail)
	phoneNumber, phoneOK := portable.NormalizePhoneNumber(legacy.PhoneNumber)
	canonical := firestoreProfile{
		FirstName: legacy.FirstName, LastName: legacy.LastName,
		ContactEmail: contactEmail, PhoneNumber: phoneNumber,
		MarketingOptIn: legacy.Marketing, TermsAccepted: true,
		CreatedAt: legacy.CreatedAt.UTC().Truncate(time.Millisecond),
		UpdatedAt: legacy.UpdatedAt.UTC().Truncate(time.Millisecond),
	}
	if !emailOK || !phoneOK {
		return MigrationBlocked, "legacy record cannot be converted without inventing or repairing domain data", nil
	}
	if _, conversionErr := toProfile(principal, canonical); conversionErr != nil {
		return MigrationBlocked, "legacy record cannot be converted without inventing or repairing domain data", nil
	}
	return MigrationRequired, "known pre-adoption record has an authorized canonical replacement", canonicalProfileMap(
		canonical,
	)
}

type legacyProfile struct {
	FirstName    string
	LastName     string
	ContactEmail string
	PhoneNumber  string
	Marketing    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func legacyProfileData(data map[string]any) (legacyProfile, bool) {
	firstName, firstNameOK := data["first_name"].(string)
	lastName, lastNameOK := data["last_name"].(string)
	contactEmail, contactEmailOK := data["contact_email"].(string)
	phoneNumber, phoneNumberOK := data["phone_number"].(string)
	marketing, marketingOK := data["marketing"].(bool)
	createdAt, createdAtOK := data["created_at"].(time.Time)
	updatedAt, updatedAtOK := data["updated_at"].(time.Time)
	if !firstNameOK || !lastNameOK || !contactEmailOK || !phoneNumberOK || !marketingOK ||
		!createdAtOK || !updatedAtOK || updatedAt.Before(createdAt) {
		return legacyProfile{}, false
	}
	return legacyProfile{
		FirstName: firstName, LastName: lastName, ContactEmail: contactEmail,
		PhoneNumber: phoneNumber, Marketing: marketing, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, true
}

func canonicalProfileData(data map[string]any, principal string) (firestoreProfile, bool) {
	if !exactCanonicalProfileFields(data) {
		return firestoreProfile{}, false
	}
	firstName, firstNameOK := data["first_name"].(string)
	lastName, lastNameOK := data["last_name"].(string)
	contactEmail, contactEmailOK := data["contact_email"].(string)
	phoneNumber, phoneNumberOK := data["phone_number"].(string)
	marketing, marketingOK := data["marketing_opt_in"].(bool)
	terms, termsOK := data["terms_accepted"].(bool)
	createdAt, createdAtOK := data["created_at"].(time.Time)
	updatedAt, updatedAtOK := data["updated_at"].(time.Time)
	stored := firestoreProfile{
		FirstName: firstName, LastName: lastName, ContactEmail: contactEmail, PhoneNumber: phoneNumber,
		MarketingOptIn: marketing, TermsAccepted: terms, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if !firstNameOK || !lastNameOK || !contactEmailOK || !phoneNumberOK || !marketingOK || !termsOK ||
		!createdAtOK || !updatedAtOK {
		return firestoreProfile{}, false
	}
	_, err := toProfile(principal, stored)
	return stored, err == nil
}

func canonicalProfileMap(stored firestoreProfile) map[string]any {
	return map[string]any{
		"first_name": stored.FirstName, "last_name": stored.LastName,
		"contact_email": stored.ContactEmail, "phone_number": stored.PhoneNumber,
		"marketing_opt_in": stored.MarketingOptIn, "terms_accepted": stored.TermsAccepted,
		"created_at": stored.CreatedAt, "updated_at": stored.UpdatedAt,
	}
}

func exactCanonicalProfileFields(data map[string]any) bool {
	return exactProfileFields(
		data, "first_name", "last_name", "contact_email", "phone_number",
		"marketing_opt_in", "terms_accepted", "created_at", "updated_at",
	)
}

func exactLegacyProfileFields(data map[string]any) bool {
	return exactProfileFields(
		data, "first_name", "last_name", "contact_email", "phone_number", "marketing", "created_at", "updated_at",
	)
}

func exactProfileFields(data map[string]any, fields ...string) bool {
	if len(data) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, exists := data[field]; !exists {
			return false
		}
	}
	return true
}

func validMigrationEvidence(value string) bool {
	if value == "" || utf8.RuneCountInString(value) > 500 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character <= 0x1f || character >= 0x7f && character <= 0x9f {
			return false
		}
	}
	return true
}

func fingerprintPrincipal(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}
