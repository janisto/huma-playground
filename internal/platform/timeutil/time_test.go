package timeutil

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
)

func TestJSONRoundTripUsesUTCMilliseconds(t *testing.T) {
	input := NewTime(time.Date(2024, 1, 15, 12, 30, 45, 123456789, time.FixedZone("EET", 2*60*60)))
	data, err := input.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if got, want := string(data), `"2024-01-15T10:30:45.123Z"`; got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}

	var output Time
	if err := output.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if got := output.UTC().Format(RFC3339Millis); got != "2024-01-15T10:30:45.123Z" {
		t.Fatalf("unexpected round trip: %s", got)
	}
}

func TestJSONNullPreservesValue(t *testing.T) {
	original := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	if err := original.UnmarshalJSON([]byte("null")); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if original.Year() != 2024 {
		t.Fatal("null unexpectedly replaced value")
	}
}

func TestJSONRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{`"not-a-time"`, `123`, `"2024-99-99T00:00:00Z"`} {
		var value Time
		if err := value.UnmarshalJSON([]byte(input)); err == nil {
			t.Fatalf("expected %s to fail", input)
		}
	}
}

func TestCBORRoundTripUsesDateTimeTag(t *testing.T) {
	input := NewTime(time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC))
	data, err := input.MarshalCBOR()
	if err != nil {
		t.Fatalf("marshal CBOR: %v", err)
	}
	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		t.Fatalf("decode tag: %v", err)
	}
	if tag.Number != 0 || tag.Content != "2024-01-15T10:30:45.123Z" {
		t.Fatalf("unexpected tag: %#v", tag)
	}

	var output Time
	if err := output.UnmarshalCBOR(data); err != nil {
		t.Fatalf("unmarshal CBOR: %v", err)
	}
	if !output.Equal(time.Date(2024, 1, 15, 10, 30, 45, 123000000, time.UTC)) {
		t.Fatalf("unexpected round trip: %s", output.Time)
	}
}

func TestCBORAcceptsBareTextAndRejectsInvalidValues(t *testing.T) {
	bare, err := cbor.Marshal("2024-01-15T10:30:45Z")
	if err != nil {
		t.Fatalf("marshal bare text: %v", err)
	}
	var value Time
	if err := value.UnmarshalCBOR(bare); err != nil {
		t.Fatalf("unmarshal bare text: %v", err)
	}

	invalidValues := [][]byte{
		nil,
		{0x01},
		append(bare, 0x01),
	}
	for _, invalid := range invalidValues {
		if err := value.UnmarshalCBOR(invalid); err == nil {
			t.Fatalf("expected %x to fail", invalid)
		}
	}
}

func TestNewTime(t *testing.T) {
	input := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if got := NewTime(input); !got.Equal(input) {
		t.Fatalf("expected %s, got %s", input, got.Time)
	}
}
