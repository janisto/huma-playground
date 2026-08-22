package pagination

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type testItem struct{ id string }

func testItems(count int) []testItem {
	items := make([]testItem, count)
	for index := range items {
		items[index].id = "item-" + strconv.Itoa(index+1)
	}
	return items
}

func paginateTestItems(items []testItem, cursor Cursor, limit int, filter string) Result[testItem] {
	query := url.Values{}
	if filter != "" {
		query.Set("category", filter)
	}
	return Paginate(items, cursor, limit, "listItems", filter, func(item testItem) string {
		return item.id
	}, "/v1/items", query)
}

func TestPaginateTraversesThreePagesForwardAndBackwardWithoutLoss(t *testing.T) {
	t.Parallel()
	items := testItems(25)
	first := paginateTestItems(items, Cursor{}, 10, "")
	assertIDs(t, first.Items, 1, 10)
	if first.PrevCursor != "" || first.NextCursor == "" {
		t.Fatalf("first navigation = next %q prev %q", first.NextCursor, first.PrevCursor)
	}

	secondCursor := mustDecode(t, first.NextCursor)
	second := paginateTestItems(items, secondCursor, 10, "")
	assertIDs(t, second.Items, 11, 20)
	if second.NextCursor == "" || second.PrevCursor == "" {
		t.Fatalf("middle navigation = next %q prev %q", second.NextCursor, second.PrevCursor)
	}

	third := paginateTestItems(items, mustDecode(t, second.NextCursor), 10, "")
	assertIDs(t, third.Items, 21, 25)
	if third.NextCursor != "" || third.PrevCursor == "" {
		t.Fatalf("terminal navigation = next %q prev %q", third.NextCursor, third.PrevCursor)
	}

	backToSecond := paginateTestItems(items, mustDecode(t, third.PrevCursor), 10, "")
	assertIDs(t, backToSecond.Items, 11, 20)
	backToFirst := paginateTestItems(items, mustDecode(t, backToSecond.PrevCursor), 10, "")
	assertIDs(t, backToFirst.Items, 1, 10)

	forward := append(append(slices.Clone(first.Items), second.Items...), third.Items...)
	if !slices.Equal(forward, items) {
		t.Fatalf("forward traversal = %#v, want %#v", forward, items)
	}
}

func TestPaginateFiltersBeforeBuildingScopedNavigation(t *testing.T) {
	t.Parallel()
	items := testItems(3)
	result := paginateTestItems(items, Cursor{}, 2, "tools")
	if result.Total != 3 || len(result.Items) != 2 {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.LinkHeader, "category=tools") || !strings.Contains(result.LinkHeader, "limit=2") {
		t.Fatalf("Link = %q", result.LinkHeader)
	}
	cursor := mustDecode(t, result.NextCursor)
	if cursor.Operation != "listItems" || cursor.Filter != "tools" || cursor.Limit != 2 ||
		cursor.Direction != "next" || cursor.Anchor != "item-2" {
		t.Fatalf("next cursor = %#v", cursor)
	}
}

func TestPaginateSingleAndEmptyPagesOmitNavigation(t *testing.T) {
	t.Parallel()
	for name, items := range map[string][]testItem{"empty": {}, "single": {{id: "one"}}} {
		t.Run(name, func(t *testing.T) {
			result := paginateTestItems(items, Cursor{}, 20, "")
			if result.LinkHeader != "" || result.NextCursor != "" || result.PrevCursor != "" {
				t.Fatalf("unexpected navigation %#v", result)
			}
		})
	}
}

func TestPaginateMaximumLimitDoesNotOverrun(t *testing.T) {
	t.Parallel()
	result := paginateTestItems(testItems(101), Cursor{}, 100, "")
	if len(result.Items) != 100 || result.NextCursor == "" {
		t.Fatalf("maximum page = %#v", result)
	}
}

func mustDecode(t *testing.T, value string) Cursor {
	t.Helper()
	cursor, err := DecodeCursor(value)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	return cursor
}

func assertIDs(t *testing.T, items []testItem, first, last int) {
	t.Helper()
	want := make([]testItem, last-first+1)
	for index := range want {
		want[index].id = "item-" + strconv.Itoa(first+index)
	}
	if !slices.Equal(items, want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}
}
