package routes

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/janisto/huma-playground/internal/pagination"
)

// Item represents a sample resource for pagination demonstration.
type Item struct {
	ID          string    `json:"id"          doc:"Unique identifier"                example:"item-001"`
	Name        string    `json:"name"        doc:"Display name"                     example:"Alpha Widget"`
	Category    string    `json:"category"    doc:"Item category"                    example:"electronics"`
	Price       float64   `json:"price"       doc:"Price in USD"                     example:"29.99"`
	InStock     bool      `json:"inStock"     doc:"Availability status"              example:"true"`
	CreatedAt   time.Time `json:"createdAt"   doc:"Creation timestamp"               example:"2024-01-15T10:30:00Z"`
	Description string    `json:"description" doc:"Detailed description of the item"`
}

// mockItems provides sample data for pagination demonstration.
var mockItems = []Item{
	{
		ID:          "item-001",
		Name:        "Alpha Widget",
		Category:    "electronics",
		Price:       29.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description: "A versatile electronic widget for everyday use",
	},
	{
		ID:          "item-002",
		Name:        "Beta Gadget",
		Category:    "electronics",
		Price:       49.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC),
		Description: "Advanced gadget with smart features",
	},
	{
		ID:          "item-003",
		Name:        "Gamma Tool",
		Category:    "tools",
		Price:       15.50,
		InStock:     false,
		CreatedAt:   time.Date(2024, 1, 17, 9, 15, 0, 0, time.UTC),
		Description: "Precision tool for professional work",
	},
	{
		ID:          "item-004",
		Name:        "Delta Component",
		Category:    "electronics",
		Price:       8.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 18, 14, 45, 0, 0, time.UTC),
		Description: "Essential component for electronics projects",
	},
	{
		ID:          "item-005",
		Name:        "Epsilon Sensor",
		Category:    "electronics",
		Price:       34.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 19, 8, 0, 0, 0, time.UTC),
		Description: "High-precision environmental sensor",
	},
	{
		ID:          "item-006",
		Name:        "Zeta Cable",
		Category:    "accessories",
		Price:       12.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 20, 16, 30, 0, 0, time.UTC),
		Description: "Premium quality data cable",
	},
	{
		ID:          "item-007",
		Name:        "Eta Adapter",
		Category:    "accessories",
		Price:       9.99,
		InStock:     false,
		CreatedAt:   time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC),
		Description: "Universal power adapter",
	},
	{
		ID:          "item-008",
		Name:        "Theta Board",
		Category:    "electronics",
		Price:       89.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 22, 11, 30, 0, 0, time.UTC),
		Description: "Development board for prototyping",
	},
	{
		ID:          "item-009",
		Name:        "Iota Switch",
		Category:    "electronics",
		Price:       5.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 23, 9, 45, 0, 0, time.UTC),
		Description: "Tactile push button switch",
	},
	{
		ID:          "item-010",
		Name:        "Kappa Display",
		Category:    "electronics",
		Price:       45.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 24, 13, 0, 0, 0, time.UTC),
		Description: "OLED display module",
	},
	{
		ID:          "item-011",
		Name:        "Lambda Motor",
		Category:    "robotics",
		Price:       24.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 25, 8, 30, 0, 0, time.UTC),
		Description: "DC motor for robotics projects",
	},
	{
		ID:          "item-012",
		Name:        "Mu Servo",
		Category:    "robotics",
		Price:       18.99,
		InStock:     false,
		CreatedAt:   time.Date(2024, 1, 26, 15, 0, 0, 0, time.UTC),
		Description: "High-torque servo motor",
	},
	{
		ID:          "item-013",
		Name:        "Nu Battery",
		Category:    "power",
		Price:       14.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 27, 10, 15, 0, 0, time.UTC),
		Description: "Rechargeable lithium battery pack",
	},
	{
		ID:          "item-014",
		Name:        "Xi Charger",
		Category:    "power",
		Price:       22.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 28, 11, 45, 0, 0, time.UTC),
		Description: "Smart battery charger",
	},
	{
		ID:          "item-015",
		Name:        "Omicron Relay",
		Category:    "electronics",
		Price:       7.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 29, 9, 0, 0, 0, time.UTC),
		Description: "5V relay module",
	},
	{
		ID:          "item-016",
		Name:        "Pi Controller",
		Category:    "electronics",
		Price:       55.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 1, 30, 14, 30, 0, 0, time.UTC),
		Description: "Microcontroller board",
	},
	{
		ID:          "item-017",
		Name:        "Rho Resistor Kit",
		Category:    "components",
		Price:       11.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC),
		Description: "Assorted resistor pack",
	},
	{
		ID:          "item-018",
		Name:        "Sigma Capacitor Set",
		Category:    "components",
		Price:       13.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 2, 10, 30, 0, 0, time.UTC),
		Description: "Electrolytic capacitor assortment",
	},
	{
		ID:          "item-019",
		Name:        "Tau LED Pack",
		Category:    "components",
		Price:       6.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 3, 11, 0, 0, 0, time.UTC),
		Description: "Multi-color LED assortment",
	},
	{
		ID:          "item-020",
		Name:        "Upsilon Wire Set",
		Category:    "accessories",
		Price:       8.99,
		InStock:     false,
		CreatedAt:   time.Date(2024, 2, 4, 9, 15, 0, 0, time.UTC),
		Description: "Jumper wire kit",
	},
	{
		ID:          "item-021",
		Name:        "Phi Breadboard",
		Category:    "tools",
		Price:       4.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 5, 13, 45, 0, 0, time.UTC),
		Description: "Solderless breadboard",
	},
	{
		ID:          "item-022",
		Name:        "Chi Soldering Iron",
		Category:    "tools",
		Price:       35.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 6, 10, 0, 0, 0, time.UTC),
		Description: "Temperature-controlled soldering station",
	},
	{
		ID:          "item-023",
		Name:        "Psi Multimeter",
		Category:    "tools",
		Price:       42.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 7, 11, 30, 0, 0, time.UTC),
		Description: "Digital multimeter with auto-ranging",
	},
	{
		ID:          "item-024",
		Name:        "Omega Oscilloscope",
		Category:    "tools",
		Price:       299.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 8, 14, 0, 0, 0, time.UTC),
		Description: "Portable digital oscilloscope",
	},
	{
		ID:          "item-025",
		Name:        "Alpha Pro Widget",
		Category:    "electronics",
		Price:       59.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 9, 8, 30, 0, 0, time.UTC),
		Description: "Professional-grade widget with extended features",
	},
	{
		ID:          "item-026",
		Name:        "Beta Max Gadget",
		Category:    "electronics",
		Price:       79.99,
		InStock:     false,
		CreatedAt:   time.Date(2024, 2, 10, 9, 0, 0, 0, time.UTC),
		Description: "Maximum performance gadget",
	},
	{
		ID:          "item-027",
		Name:        "Gamma Plus Tool",
		Category:    "tools",
		Price:       25.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 11, 10, 15, 0, 0, time.UTC),
		Description: "Enhanced precision tool",
	},
	{
		ID:          "item-028",
		Name:        "Delta Ultra Component",
		Category:    "electronics",
		Price:       16.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 12, 11, 45, 0, 0, time.UTC),
		Description: "Ultra-reliable component",
	},
	{
		ID:          "item-029",
		Name:        "Epsilon HD Sensor",
		Category:    "electronics",
		Price:       54.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 13, 13, 0, 0, 0, time.UTC),
		Description: "High-definition sensor array",
	},
	{
		ID:          "item-030",
		Name:        "Zeta Premium Cable",
		Category:    "accessories",
		Price:       19.99,
		InStock:     true,
		CreatedAt:   time.Date(2024, 2, 14, 15, 30, 0, 0, time.UTC),
		Description: "Gold-plated premium cable",
	},
}

// ItemsInput defines query parameters for listing items.
type ItemsInput struct {
	pagination.Params
	Category string `query:"category" doc:"Filter by category" example:"electronics" enum:"electronics,tools,accessories,robotics,power,components"`
}

// ItemsData is the response body containing paginated items.
type ItemsData struct {
	Items []Item `json:"items" doc:"List of items"`
	Total int    `json:"total" doc:"Total count of items matching the filter" example:"30"`
}

// ItemsOutput is the response wrapper with pagination Link header.
type ItemsOutput struct {
	Link string `header:"Link" doc:"RFC 8288 pagination links"`
	Body ItemsData
}

const itemCursorType = "item"

func registerItems(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-items",
		Method:      http.MethodGet,
		Path:        "/items",
		Summary:     "List items with cursor-based pagination",
		Description: "Returns a paginated list of items. Use the cursor from the Link header to navigate between pages.",
		Tags:        []string{"Items"},
	}, func(_ context.Context, input *ItemsInput) (*ItemsOutput, error) {
		cursor, err := pagination.DecodeCursor(input.Cursor)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid cursor format")
		}

		if cursor.Type != "" && cursor.Type != itemCursorType {
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
			itemCursorType,
			func(item Item) string { return item.ID },
			"/items",
			query,
		)

		return &ItemsOutput{
			Link: result.LinkHeader,
			Body: ItemsData{
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
	// Category is guaranteed to be lowercase by Huma's enum validation.
	return slices.DeleteFunc(slices.Clone(items), func(item Item) bool {
		return item.Category != category
	})
}

func findItemIndex(items []Item, id string) int {
	return slices.IndexFunc(items, func(item Item) bool {
		return item.ID == id
	})
}
