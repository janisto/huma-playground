package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	profilesvc "github.com/janisto/huma-playground/internal/service/profile"
)

func writeManifest(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestReadManifestDefaultsToEmptyDryRunAuthorization(t *testing.T) {
	manifest, err := readManifest("")
	if err != nil || manifest.Version != profilesvc.ProfileMigrationManifestVersion || len(manifest.Entries) != 0 {
		t.Fatalf("manifest=%#v err=%v", manifest, err)
	}
}

func TestReadManifestAcceptsOnlyStrictClosedValidatedJSON(t *testing.T) {
	valid := "{\"version\":1,\"entries\":{\"principal-a\":{\"termsAccepted\":true,\"evidence\":\"ticket-123\"}}}"
	manifest, err := readManifest(writeManifest(t, valid))
	if err != nil || !manifest.Entries["principal-a"].TermsAccepted {
		t.Fatalf("manifest=%#v err=%v", manifest, err)
	}
	tests := map[string]string{
		"duplicate member": strings.Replace(valid, "\"version\":1", "\"version\":1,\"version\":1", 1),
		"trailing data":    valid + "{}",
		"unknown member":   strings.Replace(valid, "\"version\":1", "\"version\":1,\"unknown\":true", 1),
		"unknown entry": strings.Replace(
			valid,
			"\"evidence\":\"ticket-123\"",
			"\"evidence\":\"ticket-123\",\"unknown\":true",
			1,
		),
		"false evidence": strings.Replace(valid, "\"termsAccepted\":true", "\"termsAccepted\":false", 1),
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := readManifest(writeManifest(t, document)); err == nil {
				t.Fatalf("invalid manifest accepted: %s", document)
			}
		})
	}
}

func TestRunRejectsUnsafeApplyArgumentsBeforeFirestoreInitialization(t *testing.T) {
	for name, arguments := range map[string][]string{
		"missing project":        {},
		"positional argument":    {"--project", "example", "extra"},
		"missing confirmation":   {"--project", "example", "--apply"},
		"wrong confirmation":     {"--project", "example", "--apply", "--confirm-project", "other"},
		"invalid manifest first": {"--project", "example", "--manifest", writeManifest(t, "{\"version\":2,\"entries\":{}}")},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(context.Background(), arguments); err == nil {
				t.Fatalf("unsafe arguments accepted: %v", arguments)
			}
		})
	}
}

func TestRunRejectsFirestoreEmulatorRoutingBeforeManifestOrClient(t *testing.T) {
	missingManifest := filepath.Join(t.TempDir(), "missing.json")
	hosts := map[string]string{
		"address":    "127.0.0.1:7130",
		"whitespace": " ",
	}
	commands := map[string][]string{
		"audit": {"--project", "live-project", "--manifest", missingManifest},
		"apply": {
			"--project", "live-project", "--manifest", missingManifest,
			"--apply", "--confirm-project", "live-project",
		},
	}
	for hostName, host := range hosts {
		t.Run(hostName, func(t *testing.T) {
			t.Setenv("FIRESTORE_EMULATOR_HOST", host)
			for commandName, arguments := range commands {
				t.Run(commandName, func(t *testing.T) {
					err := run(t.Context(), arguments)
					if err == nil ||
						err.Error() != "FIRESTORE_EMULATOR_HOST must be empty or unset for profile migration" {
						t.Fatalf("error=%v", err)
					}
				})
			}
		})
	}
}
