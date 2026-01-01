package pagination

import (
	"errors"
	"strings"
	"testing"
)

func TestCursorEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
	}{
		{"simple", Cursor{Type: "user", Value: "123"}},
		{"with-uuid", Cursor{Type: "order", Value: "550e8400-e29b-41d4-a716-446655440000"}},
		{"with-timestamp", Cursor{Type: "event", Value: "2024-01-15T10:30:00Z"}},
		{"with-special-chars", Cursor{Type: "item", Value: "abc/def+ghi=jkl"}},
		{"empty-value", Cursor{Type: "test", Value: ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := tc.cursor.Encode()
			decoded, err := DecodeCursor(encoded)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if decoded.Type != tc.cursor.Type {
				t.Errorf("type mismatch: got %q, want %q", decoded.Type, tc.cursor.Type)
			}
			if decoded.Value != tc.cursor.Value {
				t.Errorf("value mismatch: got %q, want %q", decoded.Value, tc.cursor.Value)
			}
		})
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	cursor, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cursor.Type != "" || cursor.Value != "" {
		t.Errorf("expected empty cursor, got %+v", cursor)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"not-base64", "!!!invalid!!!"},
		{"no-separator", "dGVzdA"},          // base64("test") - no colon
		{"invalid-base64-chars", "abc def"}, // space is invalid
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeCursor(tc.input)
			if !errors.Is(err, ErrInvalidCursor) {
				t.Errorf("expected ErrInvalidCursor, got %v", err)
			}
		})
	}
}

func TestCursorEncodeURLSafe(t *testing.T) {
	cursor := Cursor{Type: "test", Value: "value+with/special=chars"}
	encoded := cursor.Encode()

	// URL-safe Base64 should not contain + or /
	for _, c := range encoded {
		if c == '+' || c == '/' {
			t.Errorf("encoded cursor contains non-URL-safe character: %c", c)
		}
	}
}

func TestCursorEmptyTypeNonEmptyValue(t *testing.T) {
	cursor := Cursor{Type: "", Value: "some-value"}
	encoded := cursor.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != "" {
		t.Errorf("expected empty type, got %q", decoded.Type)
	}
	if decoded.Value != "some-value" {
		t.Errorf("expected 'some-value', got %q", decoded.Value)
	}
}

func TestCursorWithColonInValue(t *testing.T) {
	cursor := Cursor{Type: "item", Value: "2024-01-15T10:30:00Z"}
	encoded := cursor.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != "item" {
		t.Errorf("type mismatch: got %q", decoded.Type)
	}
	if decoded.Value != "2024-01-15T10:30:00Z" {
		t.Errorf("value mismatch: got %q", decoded.Value)
	}
}

func TestCursorWithMultipleColonsInValue(t *testing.T) {
	cursor := Cursor{Type: "composite", Value: "a:b:c:d"}
	encoded := cursor.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Value != "a:b:c:d" {
		t.Errorf("value should preserve all colons, got %q", decoded.Value)
	}
}

func TestCursorLongValue(t *testing.T) {
	longValue := strings.Repeat("x", 1000)
	cursor := Cursor{Type: "item", Value: longValue}
	encoded := cursor.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Value != longValue {
		t.Error("long value not preserved correctly")
	}
}

func TestCursorUnicodeValue(t *testing.T) {
	cursor := Cursor{Type: "item", Value: "日本語テスト"}
	encoded := cursor.Encode()

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Value != "日本語テスト" {
		t.Errorf("unicode value mismatch: got %q", decoded.Value)
	}
}

func TestDecodeCursorPaddingVariations(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
	}{
		{"no-padding-needed", Cursor{Type: "abc", Value: "def"}},
		{"one-pad", Cursor{Type: "ab", Value: "cd"}},
		{"two-pad", Cursor{Type: "a", Value: "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := tc.cursor.Encode()
			decoded, err := DecodeCursor(encoded)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if decoded.Type != tc.cursor.Type || decoded.Value != tc.cursor.Value {
				t.Errorf("mismatch: got %+v, want %+v", decoded, tc.cursor)
			}
		})
	}
}
