package pagination

import (
	"net/url"
	"strings"
	"testing"
)

func TestBuildLinkHeaderBothCursors(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{"filter": []string{"active"}}
	next := "bmV4dA"
	prev := "cHJldg"

	link := BuildLinkHeader(baseURL, query, next, prev)

	if !strings.Contains(link, `rel="next"`) {
		t.Error("missing next rel")
	}
	if !strings.Contains(link, `rel="prev"`) {
		t.Error("missing prev rel")
	}
	if !strings.Contains(link, "cursor="+next) {
		t.Error("missing next cursor")
	}
	if !strings.Contains(link, "cursor="+prev) {
		t.Error("missing prev cursor")
	}
	if !strings.Contains(link, "filter=active") {
		t.Error("original query param not preserved")
	}
}

func TestBuildLinkHeaderOnlyNext(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{}
	next := "bmV4dA"

	link := BuildLinkHeader(baseURL, query, next, "")

	if !strings.Contains(link, `rel="next"`) {
		t.Error("missing next rel")
	}
	if strings.Contains(link, `rel="prev"`) {
		t.Error("should not contain prev rel")
	}
}

func TestBuildLinkHeaderOnlyPrev(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{}
	prev := "cHJldg"

	link := BuildLinkHeader(baseURL, query, "", prev)

	if strings.Contains(link, `rel="next"`) {
		t.Error("should not contain next rel")
	}
	if !strings.Contains(link, `rel="prev"`) {
		t.Error("missing prev rel")
	}
}

func TestBuildLinkHeaderEmpty(t *testing.T) {
	link := BuildLinkHeader("https://api.example.com/items", nil, "", "")
	if link != "" {
		t.Errorf("expected empty string, got %q", link)
	}
}

func TestBuildLinkHeaderPreservesQueryParams(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{
		"filter": []string{"active"},
		"sort":   []string{"created_at"},
		"limit":  []string{"20"},
	}
	next := "bmV4dA"

	link := BuildLinkHeader(baseURL, query, next, "")

	if !strings.Contains(link, "filter=active") {
		t.Error("filter param not preserved")
	}
	if !strings.Contains(link, "sort=created_at") {
		t.Error("sort param not preserved")
	}
	if !strings.Contains(link, "limit=20") {
		t.Error("limit param not preserved")
	}
}

func TestCloneValuesNil(t *testing.T) {
	cloned := cloneValues(nil)
	if cloned == nil {
		t.Error("expected non-nil map")
	}
	if len(cloned) != 0 {
		t.Error("expected empty map")
	}
}

func TestCloneValuesIsolation(t *testing.T) {
	original := url.Values{"key": []string{"value"}}
	cloned := cloneValues(original)
	cloned.Set("key", "modified")

	if original.Get("key") != "value" {
		t.Error("original was modified")
	}
}

func TestParamsDefaultLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		expected int
	}{
		{"zero", 0, 20},
		{"negative", -1, 20},
		{"positive", 50, 50},
		{"max", 100, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Params{Limit: tc.limit}
			if got := p.DefaultLimit(); got != tc.expected {
				t.Errorf("got %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestBuildLinkHeaderURLEncoding(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{"filter": []string{"hello world"}}
	next := "bmV4dA"

	link := BuildLinkHeader(baseURL, query, next, "")

	if !strings.Contains(link, "filter=hello+world") && !strings.Contains(link, "filter=hello%20world") {
		t.Error("filter param should be URL encoded")
	}
}

func TestBuildLinkHeaderMultipleQueryValues(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{"tag": []string{"a", "b", "c"}}
	next := "bmV4dA"

	link := BuildLinkHeader(baseURL, query, next, "")

	if !strings.Contains(link, "tag=a") || !strings.Contains(link, "tag=b") || !strings.Contains(link, "tag=c") {
		t.Error("all tag values should be present")
	}
}

func TestBuildLinkHeaderCursorWithSpecialChars(t *testing.T) {
	baseURL := "https://api.example.com/items"
	cursor := Cursor{Type: "item", Value: "abc/def+ghi=jkl"}.Encode()

	link := BuildLinkHeader(baseURL, nil, cursor, "")

	if !strings.Contains(link, "cursor=") {
		t.Error("cursor param should be present")
	}
	if strings.Contains(link, "+") && !strings.Contains(link, "%2B") {
		t.Error("+ in cursor should be URL encoded")
	}
}

func TestBuildLinkHeaderReplacesExistingCursor(t *testing.T) {
	baseURL := "https://api.example.com/items"
	query := url.Values{"cursor": []string{"old-cursor"}, "limit": []string{"10"}}
	next := "new-cursor"

	link := BuildLinkHeader(baseURL, query, next, "")

	if strings.Contains(link, "old-cursor") {
		t.Error("old cursor should be replaced")
	}
	if !strings.Contains(link, "cursor=new-cursor") {
		t.Error("new cursor should be present")
	}
	if !strings.Contains(link, "limit=10") {
		t.Error("other params should be preserved")
	}
}

func TestBuildLinkHeaderEmptyBaseURL(t *testing.T) {
	link := BuildLinkHeader("", nil, "next", "")

	if !strings.Contains(link, "<?cursor=next>") {
		t.Errorf("should handle empty base URL, got %q", link)
	}
}

func TestBuildLinkHeaderRelativePath(t *testing.T) {
	link := BuildLinkHeader("/items", nil, "next", "")

	if !strings.Contains(link, "</items?cursor=next>") {
		t.Errorf("should handle relative path, got %q", link)
	}
}
