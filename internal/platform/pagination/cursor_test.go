package pagination

import (
	"encoding/base64"
	"testing"
)

func TestCursorRoundTripPreservesCompleteScope(t *testing.T) {
	t.Parallel()
	scope := Scope{
		Operation: "listGitHubRepositoryActivity", Owner: "octocat", Repo: "hello-world", Filter: "", Limit: 100,
	}
	want := NewCursor(scope, "prev", "opaque-provider-position")
	encoded := want.Encode()
	got, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode emitted cursor: %v", err)
	}
	if got != want || !got.Matches(scope) {
		t.Fatalf("decoded cursor = %#v, want %#v", got, want)
	}
	if len(encoded) > MaxCursorLength {
		t.Fatalf("emitted cursor length = %d", len(encoded))
	}
}

func TestDecodeCursorRejectsMalformedOrNonCanonicalState(t *testing.T) {
	t.Parallel()
	encode := func(raw string) string { return base64.RawURLEncoding.EncodeToString([]byte(raw)) }
	tests := map[string]string{
		"base64 padding":  NewCursor(Scope{Operation: "listItems", Limit: 20}, "next", "item-020").Encode() + "=",
		"invalid base64":  "not+a+cursor",
		"whitespace json": encode(` {"v":1,"operation":"listItems","limit":20,"direction":"next","anchor":"item-020"}`),
		"reordered json":  encode(`{"operation":"listItems","v":1,"limit":20,"direction":"next","anchor":"item-020"}`),
		"unknown member": encode(
			`{"v":1,"operation":"listItems","limit":20,"direction":"next","anchor":"item-020","extra":true}`,
		),
		"duplicate member": encode(
			`{"v":1,"operation":"listItems","operation":"listItems","limit":20,"direction":"next","anchor":"item-020"}`,
		),
		"wrong version": encode(`{"v":2,"operation":"listItems","limit":20,"direction":"next","anchor":"item-020"}`),
		"no operation":  encode(`{"v":1,"operation":"","limit":20,"direction":"next","anchor":"item-020"}`),
		"zero limit":    encode(`{"v":1,"operation":"listItems","limit":0,"direction":"next","anchor":"item-020"}`),
		"limit 101":     encode(`{"v":1,"operation":"listItems","limit":101,"direction":"next","anchor":"item-020"}`),
		"bad direction": encode(
			`{"v":1,"operation":"listItems","limit":20,"direction":"sideways","anchor":"item-020"}`,
		),
		"trailing json": encode(`{"v":1,"operation":"listItems","limit":20,"direction":"next","anchor":"item-020"}x`),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeCursor(value); err == nil {
				t.Fatal("expected invalid cursor")
			}
		})
	}
}

func TestDecodeCursorEmptySelectsFirstPage(t *testing.T) {
	t.Parallel()
	got, err := DecodeCursor("")
	if err != nil || got != (Cursor{}) {
		t.Fatalf("DecodeCursor(empty) = %#v, %v", got, err)
	}
}

func TestCursorScopeDetectsEveryResultShapingChange(t *testing.T) {
	t.Parallel()
	base := Scope{Operation: "listGitHubRepositoryTags", Owner: "octocat", Repo: "hello-world", Limit: 20}
	cursor := NewCursor(base, "next", "2")
	changes := []Scope{
		{Operation: "listGitHubRepositoryActivity", Owner: base.Owner, Repo: base.Repo, Limit: base.Limit},
		{Operation: base.Operation, Owner: "other", Repo: base.Repo, Limit: base.Limit},
		{Operation: base.Operation, Owner: base.Owner, Repo: "other", Limit: base.Limit},
		{Operation: base.Operation, Owner: base.Owner, Repo: base.Repo, Limit: 21},
		{Operation: base.Operation, Owner: base.Owner, Repo: base.Repo, Filter: "tools", Limit: base.Limit},
	}
	for _, changed := range changes {
		if cursor.Matches(changed) {
			t.Fatalf("cursor unexpectedly matched changed scope %#v", changed)
		}
	}
}

func FuzzDecodeCursor(f *testing.F) {
	f.Add(NewCursor(Scope{Operation: "listItems", Limit: 20}, "next", "item-020").Encode())
	f.Add("")
	f.Add("not-a-cursor")
	f.Fuzz(func(t *testing.T, value string) {
		cursor, err := DecodeCursor(value)
		if err == nil && value != "" && cursor.Encode() != value {
			t.Fatalf("accepted noncanonical cursor %q", value)
		}
	})
}
