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
	apiiterator "google.golang.org/api/iterator"

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
	audit, err := buildProfileMigrationAudit(manifest, func(visit func(migrationDocument) error) error {
		return visitProfileMigrationDocuments(ctx, client, visit)
	})
	if err != nil {
		return nil, err
	}
	return audit.results, nil
}

type profileMigrationAudit struct {
	results []MigrationResult
	targets []migrationTarget
}

func buildProfileMigrationAudit(
	manifest MigrationManifest,
	visitDocuments func(func(migrationDocument) error) error,
) (profileMigrationAudit, error) {
	if err := manifest.Validate(); err != nil {
		return profileMigrationAudit{}, err
	}
	audit := profileMigrationAudit{}
	seen := make(map[string]struct{})
	if err := visitDocuments(func(document migrationDocument) error {
		if document.principal == "" {
			audit.results = append(audit.results, MigrationResult{
				DocumentFingerprint: document.fingerprint,
				State:               MigrationBlocked,
				Reason:              "profile document has a noncanonical ownership key",
			})
			return nil
		}
		if _, duplicate := seen[document.principal]; duplicate {
			return errors.New("profile migration found duplicate logical principal records")
		}
		seen[document.principal] = struct{}{}
		state, reason, _ := classifyMigration(
			document.data, document.principal, manifest.Entries[document.principal],
		)
		audit.results = append(audit.results, MigrationResult{
			DocumentFingerprint: document.fingerprint,
			State:               state,
			Reason:              reason,
		})
		audit.targets = append(audit.targets, migrationTarget{
			reference: document.reference, principal: document.principal, fingerprint: document.fingerprint,
		})
		return nil
	}); err != nil {
		return profileMigrationAudit{}, err
	}
	for principal := range manifest.Entries {
		if _, exists := seen[principal]; !exists {
			audit.results = append(audit.results, MigrationResult{
				DocumentFingerprint: fingerprintPrincipal(principal),
				State:               MigrationBlocked,
				Reason:              "migration authorization has no matching profile record",
			})
		}
	}
	return audit, nil
}

// ApplyProfileMigration performs no writes unless a complete read-only audit
// has no blocked records. Each replacement is then compare-checked in a
// transaction, making the command safe to rerun after an interrupted run.
func ApplyProfileMigration(
	ctx context.Context,
	client *firestore.Client,
	manifest MigrationManifest,
) ([]MigrationResult, error) {
	return applyProfileMigration(
		manifest,
		func(visit func(migrationDocument) error) error {
			return visitProfileMigrationDocuments(ctx, client, visit)
		},
		func(target migrationTarget) (MigrationResult, error) {
			result := MigrationResult{DocumentFingerprint: target.fingerprint}
			transactionErr := client.RunTransaction(
				ctx,
				func(ctx context.Context, transaction *firestore.Transaction) error {
					current, getErr := transaction.Get(target.reference)
					if getErr != nil {
						return getErr
					}
					state, reason, replacement := classifyMigration(
						current.Data(), target.principal, manifest.Entries[target.principal],
					)
					result.State, result.Reason = state, reason
					switch state {
					case MigrationVerified:
						return nil
					case MigrationRequired:
						if setErr := transaction.Set(target.reference, replacement); setErr != nil {
							return setErr
						}
						result.State = MigrationApplied
						return nil
					default:
						return errors.New("profile record became blocked during migration")
					}
				},
			)
			return result, transactionErr
		},
	)
}

func applyProfileMigration(
	manifest MigrationManifest,
	visitDocuments func(func(migrationDocument) error) error,
	applyTarget func(migrationTarget) (MigrationResult, error),
) ([]MigrationResult, error) {
	audit, err := buildProfileMigrationAudit(manifest, visitDocuments)
	if err != nil {
		return nil, err
	}
	for _, result := range audit.results {
		if result.State == MigrationBlocked {
			return audit.results, errors.New("profile migration is blocked; no records were changed")
		}
	}
	results := make([]MigrationResult, 0, len(audit.targets))
	for _, target := range audit.targets {
		result, err := applyTarget(target)
		if err != nil {
			return results, fmt.Errorf("apply profile migration to %s: %w", target.fingerprint, err)
		}
		results = append(results, result)
	}
	return results, nil
}

type migrationDocument struct {
	reference   *firestore.DocumentRef
	data        map[string]any
	principal   string
	fingerprint string
}

type migrationTarget struct {
	reference   *firestore.DocumentRef
	principal   string
	fingerprint string
}

type migrationDocumentIterator interface {
	Next() (migrationDocument, error)
	Stop()
}

type firestoreMigrationDocumentIterator struct {
	iterator *firestore.DocumentIterator
	encoded  bool
}

func (iterator *firestoreMigrationDocumentIterator) Next() (migrationDocument, error) {
	snapshot, err := iterator.iterator.Next()
	if err != nil {
		return migrationDocument{}, err
	}
	return migrationDocumentFromSnapshot(snapshot, iterator.encoded), nil
}

func (iterator *firestoreMigrationDocumentIterator) Stop() {
	iterator.iterator.Stop()
}

func visitProfileMigrationDocuments(
	ctx context.Context,
	client *firestore.Client,
	visit func(migrationDocument) error,
) error {
	if client == nil {
		return errors.New("firestore client is required")
	}
	collections := []struct {
		collection *firestore.CollectionRef
		encoded    bool
	}{
		{collection: client.Collection(profilesCollection)},
		{
			collection: client.Collection(profilesCollection).Doc(encodedProfilesDocument).
				Collection(encodedProfilesCollection),
			encoded: true,
		},
	}
	for _, collection := range collections {
		iterator := &firestoreMigrationDocumentIterator{
			iterator: collection.collection.Documents(ctx), encoded: collection.encoded,
		}
		if err := visitMigrationDocuments(iterator, visit); err != nil {
			return err
		}
	}
	return nil
}

func visitMigrationDocuments(iterator migrationDocumentIterator, visit func(migrationDocument) error) error {
	defer iterator.Stop()
	for {
		document, err := iterator.Next()
		if errors.Is(err, apiiterator.Done) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("iterate profile documents: %w", err)
		}
		if err := visit(document); err != nil {
			return err
		}
	}
}

func migrationDocumentFromSnapshot(snapshot *firestore.DocumentSnapshot, encoded bool) migrationDocument {
	principal := migrationPrincipalForDocumentID(snapshot.Ref.ID, encoded)
	fingerprintValue := snapshot.Ref.Path
	if principal != "" {
		fingerprintValue = principal
	}
	return migrationDocument{
		reference:   snapshot.Ref,
		data:        snapshot.Data(),
		principal:   principal,
		fingerprint: fingerprintPrincipal(fingerprintValue),
	}
}

func migrationPrincipalForDocumentID(documentID string, encoded bool) string {
	principal := documentID
	relativePath := profilesCollection + "/" + documentID
	if encoded {
		decoded, err := base64.RawURLEncoding.DecodeString(documentID)
		if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != documentID {
			principal = ""
		} else {
			principal = string(decoded)
		}
		relativePath = profilesCollection + "/" + encodedProfilesDocument + "/" +
			encodedProfilesCollection + "/" + documentID
	}
	if !validProfileID(principal) || profileDocumentPath(principal) != relativePath {
		return ""
	}
	return principal
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
