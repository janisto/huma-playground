package profile

import (
	"reflect"
	"strings"
	"testing"
	"time"
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
