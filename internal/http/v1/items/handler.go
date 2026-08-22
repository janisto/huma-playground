package items

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/portable"
)

const operationID = "listItems"

// Register wires item routes into the provided API router.
func Register(api huma.API, prefix string) {
	huma.Register(api, huma.Operation{
		OperationID: operationID,
		Method:      http.MethodGet,
		Path:        "/items",
		Summary:     "List items with cursor-based pagination",
		Description: "Returns a paginated list of items. Only limit, cursor, and category are accepted; unknown or repeated query parameters are rejected.",
		Tags:        []string{"Items"},
		Security:    []map[string][]string{},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusNotAcceptable,
			http.StatusUnprocessableEntity,
			http.StatusInternalServerError,
		},
	}, func(ctx context.Context, input *ItemsListInput) (*ItemsListOutput, error) {
		cursor, err := pagination.DecodeCursor(input.Cursor)
		if err != nil {
			return nil, portable.ErrorForContext(ctx, portable.CodeInvalidRequest)
		}

		filtered := filterItems(mockItems, input.Category)

		limit := input.DefaultLimit()
		if input.Cursor != "" && (cursor.Operation != operationID || cursor.Limit != limit ||
			cursor.Filter != input.Category || cursor.Owner != "" || cursor.Repo != "" ||
			cursor.Upstream != "" || cursor.Anchor == "" || findItemIndex(filtered, cursor.Anchor) == -1) {
			return nil, portable.ErrorForContext(ctx, portable.CodeInvalidRequest)
		}

		query := url.Values{}
		if input.Category != "" {
			query.Set("category", input.Category)
		}

		result := pagination.Paginate(
			filtered,
			cursor,
			limit,
			operationID,
			input.Category,
			func(item Item) string { return item.ID },
			prefix+"/items",
			query,
		)

		return &ItemsListOutput{
			Link: result.LinkHeader,
			Body: ListData{
				Items: result.Items,
				Total: result.Total,
			},
		}, nil
	})
}

func filterItems(items []Item, category string) []Item {
	if category == "" {
		return items
	}
	return slices.DeleteFunc(slices.Clone(items), func(item Item) bool {
		return item.Category != category
	})
}

func findItemIndex(items []Item, id string) int {
	return slices.IndexFunc(items, func(item Item) bool {
		return item.ID == id
	})
}
