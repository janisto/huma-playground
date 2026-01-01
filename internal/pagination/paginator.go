package pagination

import (
	"net/url"
	"strconv"
)

// Result holds the outcome of a pagination operation.
type Result[T any] struct {
	Items      []T
	Total      int
	LinkHeader string
	NextCursor string
	PrevCursor string
}

// Paginate applies cursor-based pagination to a slice of items.
//
// Parameters:
//   - items: The full slice of items to paginate
//   - cursor: The decoded cursor from the request
//   - limit: Maximum items per page
//   - cursorType: Type identifier for cursor validation (e.g., "item", "user")
//   - getID: Function to extract the ID from an item
//   - baseURL: Base URL path for Link header (e.g., "/items")
//   - query: Additional query parameters to preserve in links
//
// Returns a Result containing the page of items and pagination metadata.
func Paginate[T any](
	items []T,
	cursor Cursor,
	limit int,
	cursorType string,
	getID func(T) string,
	baseURL string,
	query url.Values,
) Result[T] {
	total := len(items)

	startIdx := 0
	if cursor.Value != "" {
		for i, item := range items {
			if getID(item) == cursor.Value {
				startIdx = i + 1
				break
			}
		}
	}

	endIdx := min(startIdx+limit, total)

	pageItems := items[startIdx:endIdx]

	var nextCursor, prevCursor string

	if endIdx < total && len(pageItems) > 0 {
		nextCursor = Cursor{Type: cursorType, Value: getID(pageItems[len(pageItems)-1])}.Encode()
	}

	if startIdx > 0 {
		if startIdx <= limit {
			prevCursor = Cursor{Type: cursorType, Value: ""}.Encode()
		} else {
			prevLastIdx := startIdx - 1
			prevCursor = Cursor{Type: cursorType, Value: getID(items[prevLastIdx-limit])}.Encode()
		}
	}

	q := cloneValues(query)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	linkHeader := BuildLinkHeader(baseURL, q, nextCursor, prevCursor)

	return Result[T]{
		Items:      pageItems,
		Total:      total,
		LinkHeader: linkHeader,
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
	}
}
