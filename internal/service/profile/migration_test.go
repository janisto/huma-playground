package profile

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	apiiterator "google.golang.org/api/iterator"
)

func canonicalMigrationData() map[string]any {
	created := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	return map[string]any{
		"first_name": "Ada", "last_name": "Lovelace",
		"contact_email": "Ada@example.com", "phone_number": "+358401234567",
		"marketing_opt_in": false, "terms_accepted": true,
		"created_at": created, "updated_at": created,
	}
}

func legacyMigrationData() map[string]any {
	created := time.Date(2026, 7, 30, 12, 0, 0, 987_654_321, time.FixedZone("test", 2*60*60))
	return map[string]any{
		"first_name": "Ada", "last_name": "Lovelace",
		"contact_email": " Ada@EXAMPLE.COM ", "phone_number": "\t+358401234567\n",
		"marketing": true, "created_at": created, "updated_at": created.Add(time.Second),
	}
}

func TestMigrationManifestValidation(t *testing.T) {
	valid := MigrationManifest{
		Version: ProfileMigrationManifestVersion,
		Entries: map[string]MigrationAuthorization{
			"principal-a": {TermsAccepted: true, Evidence: "support-case-123"},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}
	tests := []MigrationManifest{
		{Version: 2, Entries: map[string]MigrationAuthorization{}},
		{Version: 1, Entries: nil},
		{Version: 1, Entries: map[string]MigrationAuthorization{"": {TermsAccepted: true, Evidence: "case"}}},
		{Version: 1, Entries: map[string]MigrationAuthorization{"principal": {TermsAccepted: false, Evidence: "case"}}},
		{Version: 1, Entries: map[string]MigrationAuthorization{"principal": {TermsAccepted: true, Evidence: " case"}}},
		{
			Version: 1,
			Entries: map[string]MigrationAuthorization{"principal": {TermsAccepted: true, Evidence: "case\nsecret"}},
		},
		{
			Version: 1,
			Entries: map[string]MigrationAuthorization{
				"principal": {TermsAccepted: true, Evidence: strings.Repeat("x", 501)},
			},
		},
	}
	for index, manifest := range tests {
		if err := manifest.Validate(); err == nil {
			t.Errorf("invalid manifest %d accepted: %#v", index, manifest)
		}
	}
}

func TestClassifyMigrationRecognizesExactCanonicalRecord(t *testing.T) {
	data := canonicalMigrationData()
	state, reason, replacement := classifyMigration(data, "principal-a", MigrationAuthorization{})
	if state != MigrationVerified || reason == "" || !reflect.DeepEqual(replacement, data) {
		t.Fatalf("state=%q reason=%q replacement=%#v", state, reason, replacement)
	}

	for name, mutate := range map[string]func(*testing.T, map[string]any){
		"unknown field": func(_ *testing.T, value map[string]any) { value["unknown"] = true },
		"missing terms": func(_ *testing.T, value map[string]any) { delete(value, "terms_accepted") },
		"false terms":   func(_ *testing.T, value map[string]any) { value["terms_accepted"] = false },
		"uncanonical email": func(_ *testing.T, value map[string]any) {
			value["contact_email"] = "Ada@EXAMPLE.COM"
		},
		"sub-millisecond timestamp": func(t *testing.T, value map[string]any) {
			t.Helper()
			value["updated_at"] = migrationTime(t, value["updated_at"]).Add(time.Nanosecond)
		},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := canonicalMigrationData()
			mutate(t, invalid)
			state, _, replacement := classifyMigration(invalid, "principal-a", MigrationAuthorization{
				TermsAccepted: true, Evidence: "case-123",
			})
			if state != MigrationBlocked || replacement != nil {
				t.Fatalf("state=%q replacement=%#v", state, replacement)
			}
		})
	}
}

func TestClassifyMigrationRequiresExplicitEvidenceAndProducesExactReplacement(t *testing.T) {
	legacy := legacyMigrationData()
	state, _, replacement := classifyMigration(legacy, "principal-a", MigrationAuthorization{})
	if state != MigrationBlocked || replacement != nil {
		t.Fatalf("legacy without authorization state=%q replacement=%#v", state, replacement)
	}

	state, reason, replacement := classifyMigration(legacy, "principal-a", MigrationAuthorization{
		TermsAccepted: true, Evidence: "ticket-456",
	})
	if state != MigrationRequired || reason == "" {
		t.Fatalf("state=%q reason=%q", state, reason)
	}
	if len(replacement) != 8 || replacement["contact_email"] != "Ada@example.com" ||
		replacement["phone_number"] != "+358401234567" ||
		replacement["marketing_opt_in"] != true || replacement["terms_accepted"] != true {
		t.Fatalf("replacement=%#v", replacement)
	}
	created := migrationTime(t, replacement["created_at"])
	updated := migrationTime(t, replacement["updated_at"])
	if created.Location() != time.UTC || updated.Location() != time.UTC ||
		created.Nanosecond()%int(time.Millisecond) != 0 || updated.Nanosecond()%int(time.Millisecond) != 0 {
		t.Fatalf("timestamps created=%s updated=%s", created, updated)
	}
	if _, hasLegacy := replacement["marketing"]; hasLegacy {
		t.Fatalf("legacy field survived: %#v", replacement)
	}
}

func TestClassifyMigrationNeverRepairsUnknownOrInvalidLegacyData(t *testing.T) {
	tests := map[string]func(*testing.T, map[string]any){
		"unknown field": func(_ *testing.T, value map[string]any) { value["unknown"] = true },
		"wrong type":    func(_ *testing.T, value map[string]any) { value["marketing"] = "true" },
		"bad email":     func(_ *testing.T, value map[string]any) { value["contact_email"] = "not-an-email" },
		"bad phone":     func(_ *testing.T, value map[string]any) { value["phone_number"] = "123" },
		"backward time": func(t *testing.T, value map[string]any) {
			t.Helper()
			value["updated_at"] = migrationTime(t, value["created_at"]).Add(-time.Second)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			legacy := legacyMigrationData()
			mutate(t, legacy)
			state, _, replacement := classifyMigration(legacy, "principal-a", MigrationAuthorization{
				TermsAccepted: true, Evidence: "case-123",
			})
			if state != MigrationBlocked || replacement != nil {
				t.Fatalf("state=%q replacement=%#v", state, replacement)
			}
		})
	}
}

func migrationTime(t *testing.T, value any) time.Time {
	t.Helper()
	timestamp, ok := value.(time.Time)
	if !ok {
		t.Fatalf("migration timestamp has type %T", value)
	}
	return timestamp
}

func TestMigrationFingerprintsDoNotExposePrincipal(t *testing.T) {
	first := fingerprintPrincipal("principal-a")
	second := fingerprintPrincipal("principal-b")
	if first == second || first == "principal-a" || len(first) != 16 || first != fingerprintPrincipal("principal-a") {
		t.Fatalf("fingerprints first=%q second=%q", first, second)
	}
}

type stubMigrationDocumentIterator struct {
	documents []migrationDocument
	terminal  error
	index     int
	nextCalls int
	stopped   bool
}

func (iterator *stubMigrationDocumentIterator) Next() (migrationDocument, error) {
	iterator.nextCalls++
	if iterator.index < len(iterator.documents) {
		document := iterator.documents[iterator.index]
		iterator.index++
		return document, nil
	}
	if iterator.terminal != nil {
		err := iterator.terminal
		iterator.terminal = nil
		return migrationDocument{}, err
	}
	return migrationDocument{}, apiiterator.Done
}

func (iterator *stubMigrationDocumentIterator) Stop() {
	iterator.stopped = true
}

func migrationTestDocument(principal string, data map[string]any) migrationDocument {
	return migrationDocument{
		data: data, principal: principal, fingerprint: fingerprintPrincipal(principal),
	}
}

func validEmptyMigrationManifest() MigrationManifest {
	return MigrationManifest{Version: ProfileMigrationManifestVersion, Entries: map[string]MigrationAuthorization{}}
}

func TestVisitMigrationDocumentsStreamsAndAlwaysStops(t *testing.T) {
	documents := []migrationDocument{
		migrationTestDocument("principal-a", canonicalMigrationData()),
		migrationTestDocument("principal-b", canonicalMigrationData()),
	}
	t.Run("complete", func(t *testing.T) {
		iterator := &stubMigrationDocumentIterator{documents: documents}
		var visited []string
		err := visitMigrationDocuments(iterator, func(document migrationDocument) error {
			visited = append(visited, document.principal)
			return nil
		})
		if err != nil || !reflect.DeepEqual(visited, []string{"principal-a", "principal-b"}) ||
			iterator.nextCalls != 3 || !iterator.stopped {
			t.Fatalf("visited=%v next=%d stopped=%t err=%v", visited, iterator.nextCalls, iterator.stopped, err)
		}
	})

	t.Run("iterator failure", func(t *testing.T) {
		failure := errors.New("stream failed")
		iterator := &stubMigrationDocumentIterator{documents: documents[:1], terminal: failure}
		var visits int
		err := visitMigrationDocuments(iterator, func(migrationDocument) error {
			visits++
			return nil
		})
		if !errors.Is(err, failure) || visits != 1 || iterator.nextCalls != 2 || !iterator.stopped {
			t.Fatalf("visits=%d next=%d stopped=%t err=%v", visits, iterator.nextCalls, iterator.stopped, err)
		}
	})

	t.Run("visitor failure", func(t *testing.T) {
		failure := errors.New("visitor failed")
		iterator := &stubMigrationDocumentIterator{documents: documents}
		err := visitMigrationDocuments(iterator, func(migrationDocument) error { return failure })
		if !errors.Is(err, failure) || iterator.nextCalls != 1 || !iterator.stopped {
			t.Fatalf("next=%d stopped=%t err=%v", iterator.nextCalls, iterator.stopped, err)
		}
	})
}

func TestBuildProfileMigrationAuditClassifiesStreamWithoutRetainingData(t *testing.T) {
	const documentCount = 5_000
	manifest := validEmptyMigrationManifest()
	audit, err := buildProfileMigrationAudit(manifest, func(visit func(migrationDocument) error) error {
		for index := range documentCount {
			principal := fmt.Sprintf("principal-%04d", index)
			data := canonicalMigrationData()
			if err := visit(migrationTestDocument(principal, data)); err != nil {
				return err
			}
			data["first_name"] = "mutated after visit"
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(audit.results) != documentCount || len(audit.targets) != documentCount {
		t.Fatalf("results=%d targets=%d", len(audit.results), len(audit.targets))
	}
	for index, result := range audit.results {
		if result.State != MigrationVerified || audit.targets[index].principal == "" {
			t.Fatalf("result[%d]=%#v target=%#v", index, result, audit.targets[index])
		}
	}
}

func TestApplyProfileMigrationRequiresCompleteUnblockedAuditBeforeSideEffects(t *testing.T) {
	t.Run("stream failure", func(t *testing.T) {
		streamFailure := errors.New("stream failed")
		applyCalls := 0
		results, err := applyProfileMigration(
			validEmptyMigrationManifest(),
			func(visit func(migrationDocument) error) error {
				if err := visit(migrationTestDocument("principal-a", canonicalMigrationData())); err != nil {
					return err
				}
				return streamFailure
			},
			func(migrationTarget) (MigrationResult, error) {
				applyCalls++
				return MigrationResult{}, nil
			},
		)
		if !errors.Is(err, streamFailure) || results != nil || applyCalls != 0 {
			t.Fatalf("results=%v applyCalls=%d err=%v", results, applyCalls, err)
		}
	})

	t.Run("blocked record", func(t *testing.T) {
		applyCalls := 0
		results, err := applyProfileMigration(
			validEmptyMigrationManifest(),
			func(visit func(migrationDocument) error) error {
				return visit(migrationTestDocument("principal-a", legacyMigrationData()))
			},
			func(migrationTarget) (MigrationResult, error) {
				applyCalls++
				return MigrationResult{}, nil
			},
		)
		if err == nil || len(results) != 1 || results[0].State != MigrationBlocked || applyCalls != 0 {
			t.Fatalf("results=%v applyCalls=%d err=%v", results, applyCalls, err)
		}
	})

	t.Run("complete audit", func(t *testing.T) {
		applyCalls := 0
		results, err := applyProfileMigration(
			validEmptyMigrationManifest(),
			func(visit func(migrationDocument) error) error {
				for _, principal := range []string{"principal-a", "principal-b"} {
					if err := visit(migrationTestDocument(principal, canonicalMigrationData())); err != nil {
						return err
					}
				}
				return nil
			},
			func(target migrationTarget) (MigrationResult, error) {
				applyCalls++
				return MigrationResult{
					DocumentFingerprint: target.fingerprint,
					State:               MigrationVerified,
					Reason:              "record remained canonical",
				}, nil
			},
		)
		if err != nil || len(results) != 2 || applyCalls != 2 {
			t.Fatalf("results=%v applyCalls=%d err=%v", results, applyCalls, err)
		}
	})
}

func TestProfileMigrationStreamsDirectAndEncodedFirestoreLayouts(t *testing.T) {
	store, cleanup := setupFirestoreTest(t)
	defer cleanup()

	principals := []string{"principal-direct", "firebase/user"}
	manifest := MigrationManifest{
		Version: ProfileMigrationManifestVersion,
		Entries: map[string]MigrationAuthorization{
			principals[0]: {TermsAccepted: true, Evidence: "case-direct"},
			principals[1]: {TermsAccepted: true, Evidence: "case-encoded"},
		},
	}
	for _, principal := range principals {
		if _, err := store.client.Doc(profileDocumentPath(principal)).
			Set(t.Context(), legacyMigrationData()); err != nil {
			t.Fatalf("seed %q: %v", principal, err)
		}
	}

	audit, err := AuditProfileMigration(t.Context(), store.client, manifest)
	if err != nil || migrationStateCount(audit, MigrationRequired) != len(principals) {
		t.Fatalf("audit=%v err=%v", audit, err)
	}
	applied, err := ApplyProfileMigration(t.Context(), store.client, manifest)
	if err != nil || migrationStateCount(applied, MigrationApplied) != len(principals) {
		t.Fatalf("applied=%v err=%v", applied, err)
	}

	for _, principal := range principals {
		snapshot, readErr := store.client.Doc(profileDocumentPath(principal)).Get(t.Context())
		if readErr != nil {
			t.Fatalf("read %q: %v", principal, readErr)
		}
		data := snapshot.Data()
		if data["terms_accepted"] != true || data["marketing_opt_in"] != true {
			t.Fatalf("canonical %q data=%#v", principal, data)
		}
		if _, legacy := data["marketing"]; legacy {
			t.Fatalf("legacy field remains for %q: %#v", principal, data)
		}
	}

	rerun, err := ApplyProfileMigration(t.Context(), store.client, manifest)
	if err != nil || migrationStateCount(rerun, MigrationVerified) != len(principals) {
		t.Fatalf("rerun=%v err=%v", rerun, err)
	}
}

func migrationStateCount(results []MigrationResult, state MigrationState) int {
	count := 0
	for _, result := range results {
		if result.State == state {
			count++
		}
	}
	return count
}
