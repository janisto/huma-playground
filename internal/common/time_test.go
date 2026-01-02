package common

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRFC3339MillisConstant(t *testing.T) {
	if RFC3339Millis != "2006-01-02T15:04:05.000Z" {
		t.Fatalf("unexpected RFC3339Millis value: %s", RFC3339Millis)
	}

	now := time.Now().UTC()
	formatted := now.Format(RFC3339Millis)

	if !strings.HasSuffix(formatted, "Z") {
		t.Fatalf("formatted time should end with Z: %s", formatted)
	}
	if len(formatted) != 24 {
		t.Fatalf("formatted time should be 24 chars, got %d: %s", len(formatted), formatted)
	}
	if formatted[19] != '.' {
		t.Fatalf("formatted time should have dot at position 19: %s", formatted)
	}
}

func TestTimeMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    Time
		expected string
	}{
		{
			name:     "zero milliseconds",
			input:    NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
			expected: `"2024-01-15T10:30:00.000Z"`,
		},
		{
			name:     "with milliseconds",
			input:    NewTime(time.Date(2024, 1, 15, 10, 30, 0, 123000000, time.UTC)),
			expected: `"2024-01-15T10:30:00.123Z"`,
		},
		{
			name:     "non-UTC timezone converted",
			input:    NewTime(time.Date(2024, 1, 15, 12, 30, 0, 0, time.FixedZone("CET", 2*60*60))),
			expected: `"2024-01-15T10:30:00.000Z"`,
		},
		{
			name:     "end of day",
			input:    NewTime(time.Date(2024, 12, 31, 23, 59, 59, 999000000, time.UTC)),
			expected: `"2024-12-31T23:59:59.999Z"`,
		},
		{
			name:     "start of unix epoch",
			input:    NewTime(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)),
			expected: `"1970-01-01T00:00:00.000Z"`,
		},
		{
			name:     "leap year date",
			input:    NewTime(time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)),
			expected: `"2024-02-29T12:00:00.000Z"`,
		},
		{
			name:     "nanoseconds truncated to millis",
			input:    NewTime(time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)),
			expected: `"2024-01-15T10:30:00.123Z"`,
		},
		{
			name:     "negative timezone offset",
			input:    NewTime(time.Date(2024, 1, 15, 5, 30, 0, 0, time.FixedZone("EST", -5*60*60))),
			expected: `"2024-01-15T10:30:00.000Z"`,
		},
		{
			name:     "midnight UTC",
			input:    NewTime(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)),
			expected: `"2024-06-15T00:00:00.000Z"`,
		},
		{
			name:     "one millisecond",
			input:    NewTime(time.Date(2024, 1, 1, 0, 0, 0, 1000000, time.UTC)),
			expected: `"2024-01-01T00:00:00.001Z"`,
		},
		{
			name:     "999 milliseconds",
			input:    NewTime(time.Date(2024, 1, 1, 0, 0, 0, 999000000, time.UTC)),
			expected: `"2024-01-01T00:00:00.999Z"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(data) != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestTimeMarshalJSONZeroValue(t *testing.T) {
	var zero Time
	data, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `"0001-01-01T00:00:00.000Z"` {
		t.Fatalf("unexpected zero time output: %s", string(data))
	}
}

func TestTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "RFC3339 with Z",
			input:    `"2024-01-15T10:30:00Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "RFC3339 with milliseconds",
			input:    `"2024-01-15T10:30:00.123Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 123000000, time.UTC),
		},
		{
			name:     "RFC3339 with nanoseconds",
			input:    `"2024-01-15T10:30:00.123456789Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC),
		},
		{
			name:     "RFC3339 with positive offset",
			input:    `"2024-01-15T12:30:00+02:00"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "RFC3339 with negative offset",
			input:    `"2024-01-15T05:30:00-05:00"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "RFC3339 midnight",
			input:    `"2024-01-15T00:00:00Z"`,
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "RFC3339 end of day",
			input:    `"2024-01-15T23:59:59Z"`,
			expected: time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC),
		},
		{
			name:     "RFC3339 with .000Z suffix",
			input:    `"2024-01-15T10:30:00.000Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "RFC3339 single digit millis",
			input:    `"2024-01-15T10:30:00.1Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 100000000, time.UTC),
		},
		{
			name:     "RFC3339 two digit millis",
			input:    `"2024-01-15T10:30:00.12Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 120000000, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Time
			if err := json.Unmarshal([]byte(tt.input), &result); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.UTC().Equal(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result.UTC())
			}
		})
	}
}

func TestTimeUnmarshalJSONNull(t *testing.T) {
	var result Time
	if err := json.Unmarshal([]byte("null"), &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected zero time, got %v", result)
	}
}

func TestTimeUnmarshalJSONPreservesExistingOnNull(t *testing.T) {
	result := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	original := result.Time

	if err := json.Unmarshal([]byte("null"), &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Equal(original) {
		t.Fatalf("null should preserve existing value, got %v", result)
	}
}

func TestTimeUnmarshalJSONInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"not a date", `"not-a-date"`},
		{"empty string", `""`},
		{"number", `12345`},
		{"boolean", `true`},
		{"invalid format", `"2024/01/15 10:30:00"`},
		{"missing time", `"2024-01-15"`},
		{"missing timezone", `"2024-01-15T10:30:00"`},
		{"invalid month", `"2024-13-15T10:30:00Z"`},
		{"invalid day", `"2024-01-32T10:30:00Z"`},
		{"invalid hour", `"2024-01-15T25:30:00Z"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Time
			err := json.Unmarshal([]byte(tt.input), &result)
			if err == nil {
				t.Fatalf("expected error for input %s", tt.input)
			}
		})
	}
}

func TestTimeRoundTrip(t *testing.T) {
	original := NewTime(time.Date(2024, 6, 15, 14, 30, 45, 123000000, time.UTC))

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed Time
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	originalTruncated := original.Truncate(time.Millisecond)
	parsedTruncated := parsed.Truncate(time.Millisecond)
	if !parsedTruncated.Equal(originalTruncated) {
		t.Fatalf("round-trip failed: original %v, parsed %v", original, parsed)
	}
}

func TestTimeInStruct(t *testing.T) {
	type Item struct {
		ID        string `json:"id"`
		CreatedAt Time   `json:"createdAt"`
	}

	item := Item{
		ID:        "test-001",
		CreatedAt: NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"id":"test-001","createdAt":"2024-01-15T10:30:00.000Z"}`
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, string(data))
	}

	var parsed Item
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.ID != item.ID {
		t.Fatalf("expected ID %s, got %s", item.ID, parsed.ID)
	}
	if !parsed.CreatedAt.Equal(item.CreatedAt.Time) {
		t.Fatalf("expected CreatedAt %v, got %v", item.CreatedAt, parsed.CreatedAt)
	}
}

func TestTimeInStructWithPointer(t *testing.T) {
	type Item struct {
		ID        string `json:"id"`
		CreatedAt *Time  `json:"createdAt,omitempty"`
	}

	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	item := Item{
		ID:        "test-001",
		CreatedAt: &ts,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"id":"test-001","createdAt":"2024-01-15T10:30:00.000Z"}`
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, string(data))
	}

	itemNil := Item{ID: "test-002"}
	dataNil, err := json.Marshal(itemNil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedNil := `{"id":"test-002"}`
	if string(dataNil) != expectedNil {
		t.Fatalf("expected %s, got %s", expectedNil, string(dataNil))
	}
}

func TestTimeInSlice(t *testing.T) {
	times := []Time{
		NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		NewTime(time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC)),
		NewTime(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)),
	}

	data, err := json.Marshal(times)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `["2024-01-01T00:00:00.000Z","2024-06-15T12:30:00.000Z","2024-12-31T23:59:59.000Z"]`
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, string(data))
	}

	var parsed []Time
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed) != len(times) {
		t.Fatalf("expected %d items, got %d", len(times), len(parsed))
	}
	for i, ts := range times {
		if !parsed[i].Equal(ts.Time) {
			t.Fatalf("item %d mismatch: expected %v, got %v", i, ts, parsed[i])
		}
	}
}

func TestTimeInMap(t *testing.T) {
	m := map[string]Time{
		"start": NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		"end":   NewTime(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)),
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]Time
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed) != len(m) {
		t.Fatalf("expected %d items, got %d", len(m), len(parsed))
	}
	for k, v := range m {
		if !parsed[k].Equal(v.Time) {
			t.Fatalf("key %s mismatch: expected %v, got %v", k, v, parsed[k])
		}
	}
}

func TestNow(t *testing.T) {
	before := time.Now()
	result := Now()
	after := time.Now()

	if result.Before(before) || result.After(after) {
		t.Fatalf("Now() returned time outside expected range")
	}
}

func TestNewTime(t *testing.T) {
	input := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	result := NewTime(input)
	if !result.Equal(input) {
		t.Fatalf("expected %v, got %v", input, result)
	}
}

func TestTimeMethodsAccessible(t *testing.T) {
	ts := NewTime(time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC))

	if ts.Year() != 2024 {
		t.Fatalf("expected year 2024, got %d", ts.Year())
	}
	if ts.Month() != time.June {
		t.Fatalf("expected month June, got %v", ts.Month())
	}
	if ts.Day() != 15 {
		t.Fatalf("expected day 15, got %d", ts.Day())
	}
	if ts.Hour() != 14 {
		t.Fatalf("expected hour 14, got %d", ts.Hour())
	}
	if ts.Minute() != 30 {
		t.Fatalf("expected minute 30, got %d", ts.Minute())
	}
	if ts.Second() != 45 {
		t.Fatalf("expected second 45, got %d", ts.Second())
	}
	if ts.Weekday() != time.Saturday {
		t.Fatalf("expected Saturday, got %v", ts.Weekday())
	}
}

func TestTimeComparison(t *testing.T) {
	t1 := NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	t2 := NewTime(time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC))
	t3 := NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	if !t1.Before(t2.Time) {
		t.Fatal("t1 should be before t2")
	}
	if !t2.After(t1.Time) {
		t.Fatal("t2 should be after t1")
	}
	if !t1.Equal(t3.Time) {
		t.Fatal("t1 should equal t3")
	}
}

func TestTimeAdd(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))

	added := ts.Add(time.Hour)
	expected := time.Date(2024, 1, 15, 11, 30, 0, 0, time.UTC)
	if !added.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, added)
	}
}

func TestTimeSub(t *testing.T) {
	t1 := NewTime(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	t2 := NewTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC))

	diff := t1.Sub(t2.Time)
	if diff != 2*time.Hour {
		t.Fatalf("expected 2h, got %v", diff)
	}
}

func TestTimeUnix(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))

	unix := ts.Unix()
	if unix != 1705314600 {
		t.Fatalf("expected unix 1705314600, got %d", unix)
	}

	unixMilli := ts.UnixMilli()
	if unixMilli != 1705314600000 {
		t.Fatalf("expected unix milli 1705314600000, got %d", unixMilli)
	}
}

func TestTimeFormat(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))

	formatted := ts.Format(RFC3339Millis)
	if formatted != "2024-01-15T10:30:00.000Z" {
		t.Fatalf("expected 2024-01-15T10:30:00.000Z, got %s", formatted)
	}

	customFormat := ts.Format("2006-01-02")
	if customFormat != "2024-01-15" {
		t.Fatalf("expected 2024-01-15, got %s", customFormat)
	}
}

func TestTimeIsZero(t *testing.T) {
	var zero Time
	if !zero.IsZero() {
		t.Fatal("zero value should report IsZero true")
	}

	nonZero := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
	if nonZero.IsZero() {
		t.Fatal("non-zero value should report IsZero false")
	}
}

func TestTimeLocation(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("EST", -5*60*60)))

	utc := ts.UTC()
	if utc.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", utc.Location())
	}
	if utc.Hour() != 15 {
		t.Fatalf("expected UTC hour 15, got %d", utc.Hour())
	}
}

func TestTimeTruncate(t *testing.T) {
	ts := NewTime(time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC))

	truncated := ts.Truncate(time.Second)
	if truncated.Nanosecond() != 0 {
		t.Fatalf("expected 0 nanoseconds, got %d", truncated.Nanosecond())
	}

	truncatedMilli := ts.Truncate(time.Millisecond)
	if truncatedMilli.Nanosecond() != 123000000 {
		t.Fatalf("expected 123000000 nanoseconds, got %d", truncatedMilli.Nanosecond())
	}
}

func TestTimeNestedStruct(t *testing.T) {
	type Metadata struct {
		CreatedAt Time `json:"createdAt"`
		UpdatedAt Time `json:"updatedAt"`
	}
	type Item struct {
		ID       string   `json:"id"`
		Metadata Metadata `json:"metadata"`
	}

	item := Item{
		ID: "test-001",
		Metadata: Metadata{
			CreatedAt: NewTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			UpdatedAt: NewTime(time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)),
		},
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"id":"test-001","metadata":{"createdAt":"2024-01-15T10:00:00.000Z","updatedAt":"2024-06-15T14:30:00.000Z"}}`
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, string(data))
	}

	var parsed Item
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.Metadata.CreatedAt.Equal(item.Metadata.CreatedAt.Time) {
		t.Fatalf("CreatedAt mismatch")
	}
	if !parsed.Metadata.UpdatedAt.Equal(item.Metadata.UpdatedAt.Time) {
		t.Fatalf("UpdatedAt mismatch")
	}
}
