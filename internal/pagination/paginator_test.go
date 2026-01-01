package pagination

import (
	"net/url"
	"testing"
)

type testItem struct {
	ID   string
	Name string
}

func TestPaginateFirstPage(t *testing.T) {
	items := makeTestItems(30)

	result := Paginate(
		items,
		Cursor{},
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if len(result.Items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(result.Items))
	}
	if result.Total != 30 {
		t.Fatalf("expected total 30, got %d", result.Total)
	}
	if result.Items[0].ID != "item-001" {
		t.Fatalf("expected first item to be item-001, got %s", result.Items[0].ID)
	}
	if result.NextCursor == "" {
		t.Fatal("expected next cursor")
	}
	if result.PrevCursor != "" {
		t.Fatalf("expected no prev cursor, got %s", result.PrevCursor)
	}
}

func TestPaginateMiddlePage(t *testing.T) {
	items := makeTestItems(30)

	cursor := Cursor{Type: "test", Value: "item-010"}
	result := Paginate(
		items,
		cursor,
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if len(result.Items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(result.Items))
	}
	if result.Items[0].ID != "item-011" {
		t.Fatalf("expected first item to be item-011, got %s", result.Items[0].ID)
	}
	if result.NextCursor == "" {
		t.Fatal("expected next cursor")
	}
	if result.PrevCursor == "" {
		t.Fatal("expected prev cursor")
	}
}

func TestPaginateLastPage(t *testing.T) {
	items := makeTestItems(30)

	cursor := Cursor{Type: "test", Value: "item-020"}
	result := Paginate(
		items,
		cursor,
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if len(result.Items) != 10 {
		t.Fatalf("expected 10 items, got %d", len(result.Items))
	}
	if result.Items[0].ID != "item-021" {
		t.Fatalf("expected first item to be item-021, got %s", result.Items[0].ID)
	}
	if result.NextCursor != "" {
		t.Fatalf("expected no next cursor, got %s", result.NextCursor)
	}
	if result.PrevCursor == "" {
		t.Fatal("expected prev cursor")
	}
}

func TestPaginateEmptyItems(t *testing.T) {
	var items []testItem

	result := Paginate(
		items,
		Cursor{},
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result.Items))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
	if result.NextCursor != "" {
		t.Fatalf("expected no next cursor, got %s", result.NextCursor)
	}
	if result.PrevCursor != "" {
		t.Fatalf("expected no prev cursor, got %s", result.PrevCursor)
	}
}

func TestPaginateWithQueryParams(t *testing.T) {
	items := makeTestItems(30)

	query := url.Values{}
	query.Set("category", "electronics")

	result := Paginate(
		items,
		Cursor{},
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		query,
	)

	if result.LinkHeader == "" {
		t.Fatal("expected link header")
	}
	if !containsString(result.LinkHeader, "category=electronics") {
		t.Fatalf("expected category in link header, got %s", result.LinkHeader)
	}
	if !containsString(result.LinkHeader, "limit=10") {
		t.Fatalf("expected limit in link header, got %s", result.LinkHeader)
	}
}

func TestPaginateCursorNotFound(t *testing.T) {
	items := makeTestItems(10)

	cursor := Cursor{Type: "test", Value: "nonexistent"}
	result := Paginate(
		items,
		cursor,
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if len(result.Items) != 10 {
		t.Fatalf("expected 10 items when cursor not found (starts from beginning), got %d", len(result.Items))
	}
	if result.Items[0].ID != "item-001" {
		t.Fatalf("expected to start from beginning, got %s", result.Items[0].ID)
	}
}

func TestPaginatePrevCursorFirstPage(t *testing.T) {
	items := makeTestItems(30)

	cursor := Cursor{Type: "test", Value: "item-010"}
	result := Paginate(
		items,
		cursor,
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if result.PrevCursor == "" {
		t.Fatal("expected prev cursor for page 2")
	}

	prevDecoded, err := DecodeCursor(result.PrevCursor)
	if err != nil {
		t.Fatalf("failed to decode prev cursor: %v", err)
	}
	if prevDecoded.Value != "" {
		t.Fatalf("expected empty prev cursor value for going back to page 1, got %s", prevDecoded.Value)
	}
}

func TestPaginatePrevCursorThirdPage(t *testing.T) {
	items := makeTestItems(30)

	cursor := Cursor{Type: "test", Value: "item-020"}
	result := Paginate(
		items,
		cursor,
		10,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if result.PrevCursor == "" {
		t.Fatal("expected prev cursor for page 3")
	}

	prevDecoded, err := DecodeCursor(result.PrevCursor)
	if err != nil {
		t.Fatalf("failed to decode prev cursor: %v", err)
	}
	if prevDecoded.Value != "item-010" {
		t.Fatalf("expected prev cursor to point to item-010, got %s", prevDecoded.Value)
	}
}

func TestPaginateLimitLargerThanItems(t *testing.T) {
	items := makeTestItems(5)

	result := Paginate(
		items,
		Cursor{},
		20,
		"test",
		func(i testItem) string { return i.ID },
		"/items",
		nil,
	)

	if len(result.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(result.Items))
	}
	if result.NextCursor != "" {
		t.Fatalf("expected no next cursor, got %s", result.NextCursor)
	}
	if result.PrevCursor != "" {
		t.Fatalf("expected no prev cursor, got %s", result.PrevCursor)
	}
}

func makeTestItems(count int) []testItem {
	items := make([]testItem, count)
	for i := range count {
		items[i] = testItem{
			ID:   "item-" + padNumber(i+1),
			Name: "Item " + padNumber(i+1),
		}
	}
	return items
}

func padNumber(n int) string {
	if n < 10 {
		return "00" + itoa(n)
	}
	if n < 100 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
