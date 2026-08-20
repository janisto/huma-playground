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

// Paginate applies scoped next/previous cursor pagination to a stable slice.
func Paginate[T any](
	items []T,
	cursor Cursor,
	limit int,
	operation string,
	filter string,
	getID func(T) string,
	baseURL string,
	query url.Values,
) Result[T] {
	total := len(items)

	startIdx := 0
	if cursor.Anchor != "" {
		for i, item := range items {
			if getID(item) == cursor.Anchor {
				if cursor.Direction == "next" {
					startIdx = i + 1
				} else {
					startIdx = max(0, i-limit)
				}
				break
			}
		}
	}

	endIdx := min(startIdx+limit, total)

	pageItems := items[startIdx:endIdx]

	var nextCursor, prevCursor string

	if endIdx < total && len(pageItems) > 0 {
		nextCursor = Cursor{
			Version: 1, Operation: operation, Limit: limit, Filter: filter,
			Direction: "next", Anchor: getID(pageItems[len(pageItems)-1]),
		}.Encode()
	}

	if startIdx > 0 && len(pageItems) > 0 {
		prevCursor = Cursor{
			Version: 1, Operation: operation, Limit: limit, Filter: filter,
			Direction: "prev", Anchor: getID(pageItems[0]),
		}.Encode()
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
