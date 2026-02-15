package items

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/platform/pagination"
)

const cursorType = "item"

// Register wires item routes into the provided API router.
func Register(api huma.API, prefix string) {
	huma.Register(api, huma.Operation{
		OperationID: "list-items",
		Method:      http.MethodGet,
		Path:        "/items",
		Summary:     "List items with cursor-based pagination",
		Description: "Returns a paginated list of items. Use the cursor from the Link header to navigate between pages.",
		Tags:        []string{"Items"},
	}, func(_ context.Context, input *ItemsListInput) (*ItemsListOutput, error) {
		cursor, err := pagination.DecodeCursor(input.Cursor)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid cursor format")
		}

		if cursor.Type != "" && cursor.Type != cursorType {
			return nil, huma.Error400BadRequest("cursor type mismatch")
		}

		filtered := filterItems(mockItems, input.Category)

		if cursor.Value != "" && findItemIndex(filtered, cursor.Value) == -1 {
			return nil, huma.Error400BadRequest("cursor references unknown item")
		}

		query := url.Values{}
		if input.Category != "" {
			query.Set("category", input.Category)
		}

		result := pagination.Paginate(
			filtered,
			cursor,
			input.DefaultLimit(),
			cursorType,
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
