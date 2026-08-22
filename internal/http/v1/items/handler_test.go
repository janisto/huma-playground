package items

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"

	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/portable"
)

func newItemsTestRouter(t *testing.T) http.Handler {
	t.Helper()
	portable.ConfigureHuma()
	config := huma.DefaultConfig("Items test", "test")
	config.DocsPath = ""
	config.OpenAPIPath = ""
	config.SchemasPath = ""
	config.CreateHooks = nil
	config.Transformers = nil
	config.Formats = portable.Formats()
	config.DefaultFormat = portable.MediaTypeJSON
	config.NoFormatFallback = true
	router := chi.NewRouter()
	router.Use(portable.RequestPolicy("/v1"))
	api := humachi.New(router, config)
	group := huma.NewGroup(api, "/v1")
	Register(group, "/v1")
	return router
}

func itemsRequest(t *testing.T, router http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeItemsPage(t *testing.T, response *httptest.ResponseRecorder) ListData {
	t.Helper()
	var page ListData
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page: %v; body=%s", err, response.Body.String())
	}
	return page
}

func linkTargets(t *testing.T, field string) map[string]string {
	t.Helper()
	targets := make(map[string]string)
	if field == "" {
		return targets
	}
	for member := range strings.SplitSeq(field, ", ") {
		openIndex, closeIndex := strings.IndexByte(member, '<'), strings.IndexByte(member, '>')
		if openIndex != 0 || closeIndex < 2 {
			t.Fatalf("invalid Link member %q", member)
		}
		switch {
		case strings.Contains(member, `rel="next"`):
			targets["next"] = member[1:closeIndex]
		case strings.Contains(member, `rel="prev"`):
			targets["prev"] = member[1:closeIndex]
		default:
			t.Fatalf("unknown Link relation %q", member)
		}
	}
	return targets
}

func pageIDs(page ListData) []string {
	ids := make([]string, len(page.Items))
	for index := range page.Items {
		ids[index] = page.Items[index].ID
	}
	return ids
}

func TestCatalogMatchesPortableContractExactly(t *testing.T) {
	expected := strings.Split(strings.TrimSpace(`
item-001|Alpha Widget|electronics|2999|true|2024-01-15T10:30:00.000Z|A versatile electronic widget for everyday use
item-002|Beta Gadget|electronics|4999|true|2024-01-16T11:00:00.000Z|Advanced gadget with smart features
item-003|Gamma Tool|tools|1550|false|2024-01-17T09:15:00.000Z|Precision tool for professional work
item-004|Delta Component|electronics|899|true|2024-01-18T14:45:00.000Z|Essential component for electronics projects
item-005|Epsilon Sensor|electronics|3499|true|2024-01-19T08:00:00.000Z|High-precision environmental sensor
item-006|Zeta Cable|accessories|1299|true|2024-01-20T16:30:00.000Z|Premium quality data cable
item-007|Eta Adapter|accessories|999|false|2024-01-21T10:00:00.000Z|Universal power adapter
item-008|Theta Board|electronics|8999|true|2024-01-22T11:30:00.000Z|Development board for prototyping
item-009|Iota Switch|electronics|599|true|2024-01-23T09:45:00.000Z|Tactile push button switch
item-010|Kappa Display|electronics|4599|true|2024-01-24T13:00:00.000Z|OLED display module
item-011|Lambda Motor|robotics|2499|true|2024-01-25T08:30:00.000Z|DC motor for robotics projects
item-012|Mu Servo|robotics|1899|false|2024-01-26T15:00:00.000Z|High-torque servo motor
item-013|Nu Battery|power|1499|true|2024-01-27T10:15:00.000Z|Rechargeable lithium battery pack
item-014|Xi Charger|power|2299|true|2024-01-28T11:45:00.000Z|Smart battery charger
item-015|Omicron Relay|electronics|799|true|2024-01-29T09:00:00.000Z|5V relay module
item-016|Pi Controller|electronics|5599|true|2024-01-30T14:30:00.000Z|Microcontroller board
item-017|Rho Resistor Kit|components|1199|true|2024-02-01T08:00:00.000Z|Assorted resistor pack
item-018|Sigma Capacitor Set|components|1399|true|2024-02-02T10:30:00.000Z|Electrolytic capacitor assortment
item-019|Tau LED Pack|components|699|true|2024-02-03T11:00:00.000Z|Multi-color LED assortment
item-020|Upsilon Wire Set|accessories|899|false|2024-02-04T09:15:00.000Z|Jumper wire kit
item-021|Phi Breadboard|tools|499|true|2024-02-05T13:45:00.000Z|Solderless breadboard
item-022|Chi Soldering Iron|tools|3599|true|2024-02-06T10:00:00.000Z|Temperature-controlled soldering station
item-023|Psi Multimeter|tools|4299|true|2024-02-07T11:30:00.000Z|Digital multimeter with auto-ranging
item-024|Omega Oscilloscope|tools|29999|true|2024-02-08T14:00:00.000Z|Portable digital oscilloscope
item-025|Alpha Pro Widget|electronics|5999|true|2024-02-09T08:30:00.000Z|Professional-grade widget with extended features
item-026|Beta Max Gadget|electronics|7999|false|2024-02-10T09:00:00.000Z|Maximum performance gadget
item-027|Gamma Plus Tool|tools|2599|true|2024-02-11T10:15:00.000Z|Enhanced precision tool
item-028|Delta Ultra Component|electronics|1699|true|2024-02-12T11:45:00.000Z|Ultra-reliable component
item-029|Epsilon HD Sensor|electronics|5499|true|2024-02-13T13:00:00.000Z|High-definition sensor array
item-030|Zeta Premium Cable|accessories|1999|true|2024-02-14T15:30:00.000Z|Gold-plated premium cable
`), "\n")
	if len(mockItems) != len(expected) {
		t.Fatalf("catalog length=%d want=%d", len(mockItems), len(expected))
	}
	for index, item := range mockItems {
		got := fmt.Sprintf("%s|%s|%s|%d|%t|%s|%s",
			item.ID, item.Name, item.Category, item.Price.AmountMinor, item.InStock,
			item.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"), item.Description,
		)
		if item.Price.Currency != "USD" || got != expected[index] {
			t.Errorf("item %d = %q currency=%q; want %q USD", index, got, item.Price.Currency, expected[index])
		}
	}
}

func TestItemsDefaultAndFilteredPages(t *testing.T) {
	router := newItemsTestRouter(t)
	response := itemsRequest(t, router, "/v1/items")
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf(
			"status=%d content-type=%q body=%s",
			response.Code,
			response.Header().Get("Content-Type"),
			response.Body.String(),
		)
	}
	page := decodeItemsPage(t, response)
	if page.Total != 30 || len(page.Items) != 20 || page.Items[0].ID != "item-001" || page.Items[19].ID != "item-020" {
		t.Fatalf("default page = %#v", page)
	}
	if targets := linkTargets(t, response.Header().Get("Link")); targets["next"] == "" || targets["prev"] != "" {
		t.Fatalf("default links = %#v", targets)
	}
	var object map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	if len(object) != 2 || object["$schema"] != nil {
		t.Fatalf("response envelope is not closed: %#v", object)
	}

	expectedTotals := map[string]int{
		"electronics": 13, "tools": 6, "accessories": 4, "robotics": 2, "power": 2, "components": 3,
	}
	for category, total := range expectedTotals {
		t.Run(category, func(t *testing.T) {
			response := itemsRequest(t, router, "/v1/items?category="+category+"&limit=100")
			page := decodeItemsPage(t, response)
			if response.Code != http.StatusOK || page.Total != total || len(page.Items) != total ||
				response.Header().Get("Link") != "" {
				t.Fatalf("status=%d page=%#v Link=%q", response.Code, page, response.Header().Get("Link"))
			}
			for _, item := range page.Items {
				if item.Category != category {
					t.Fatalf("wrong category item: %#v", item)
				}
			}
		})
	}
}

func TestItemsTraversesThreePagesBothDirectionsWithoutGaps(t *testing.T) {
	router := newItemsTestRouter(t)
	target := "/v1/items?limit=10"
	forward := make([]string, 0, 30)
	var middlePrev, terminalPrev string
	for pageNumber := 1; pageNumber <= 3; pageNumber++ {
		response := itemsRequest(t, router, target)
		if response.Code != http.StatusOK {
			t.Fatalf("page %d status=%d body=%s", pageNumber, response.Code, response.Body.String())
		}
		page := decodeItemsPage(t, response)
		forward = append(forward, pageIDs(page)...)
		targets := linkTargets(t, response.Header().Get("Link"))
		switch pageNumber {
		case 1:
			if targets["prev"] != "" || targets["next"] == "" {
				t.Fatalf("first links = %#v", targets)
			}
			target = targets["next"]
		case 2:
			if targets["prev"] == "" || targets["next"] == "" {
				t.Fatalf("middle links = %#v", targets)
			}
			middlePrev = targets["prev"]
			target = targets["next"]
		case 3:
			if targets["prev"] == "" || targets["next"] != "" {
				t.Fatalf("terminal links = %#v", targets)
			}
			terminalPrev = targets["prev"]
		}
	}
	for index, id := range forward {
		if id != fmt.Sprintf("item-%03d", index+1) {
			t.Fatalf("forward item %d = %q", index, id)
		}
	}

	middleResponse := itemsRequest(t, router, terminalPrev)
	if got := pageIDs(decodeItemsPage(t, middleResponse)); strings.Join(got, ",") != strings.Join(forward[10:20], ",") {
		t.Fatalf("back to middle = %v", got)
	}
	firstResponse := itemsRequest(t, router, middlePrev)
	if got := pageIDs(decodeItemsPage(t, firstResponse)); strings.Join(got, ",") != strings.Join(forward[:10], ",") {
		t.Fatalf("back to first = %v", got)
	}
}

func TestItemsRejectsMalformedQueryAndCursorScopes(t *testing.T) {
	wrongOperation := pagination.NewCursor(pagination.Scope{Operation: "other", Limit: 10}, "next", "item-010").Encode()
	wrongLimit := pagination.NewCursor(pagination.Scope{Operation: operationID, Limit: 20}, "next", "item-010").Encode()
	wrongFilter := pagination.NewCursor(pagination.Scope{Operation: operationID, Limit: 10, Filter: "tools"}, "next", "item-003").
		Encode()
	stale := pagination.NewCursor(pagination.Scope{Operation: operationID, Limit: 10}, "next", "item-999").Encode()
	noncanonical := pagination.NewCursor(pagination.Scope{Operation: operationID, Limit: 10}, "next", "item-010").
		Encode() +
		"="
	tests := []struct {
		name, query, code string
		status            int
	}{
		{name: "unknown", query: "page=2", status: 400, code: "invalid_request"},
		{name: "repeated", query: "limit=1&limit=2", status: 400, code: "invalid_request"},
		{name: "empty cursor", query: "cursor=", status: 400, code: "invalid_request"},
		{name: "malformed percent", query: "cursor=%ZZ", status: 400, code: "invalid_request"},
		{name: "invalid utf8", query: "cursor=%FF", status: 400, code: "invalid_request"},
		{name: "signed limit", query: "limit=%2B1", status: 422, code: "validation_failed"},
		{name: "zero", query: "limit=0", status: 422, code: "validation_failed"},
		{name: "maximum plus one", query: "limit=101", status: 422, code: "validation_failed"},
		{name: "overflow", query: "limit=18446744073709551616", status: 422, code: "validation_failed"},
		{name: "unknown category", query: "category=other", status: 422, code: "validation_failed"},
		{name: "malformed cursor", query: "limit=10&cursor=nope", status: 400, code: "invalid_request"},
		{
			name:   "noncanonical cursor",
			query:  "limit=10&cursor=" + url.QueryEscape(noncanonical),
			status: 400,
			code:   "invalid_request",
		},
		{name: "wrong operation", query: "limit=10&cursor=" + wrongOperation, status: 400, code: "invalid_request"},
		{name: "changed limit", query: "limit=10&cursor=" + wrongLimit, status: 400, code: "invalid_request"},
		{
			name:   "changed filter",
			query:  "limit=10&category=electronics&cursor=" + wrongFilter,
			status: 400,
			code:   "invalid_request",
		},
		{name: "stale anchor", query: "limit=10&cursor=" + stale, status: 400, code: "invalid_request"},
		{
			name:   "oversized cursor",
			query:  "cursor=" + strings.Repeat("a", pagination.MaxCursorLength+1),
			status: 400,
			code:   "invalid_request",
		},
	}
	router := newItemsTestRouter(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/items", nil)
			request.URL.RawQuery = test.query
			request.Header.Set("Accept", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
			var problem portable.ProblemError
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil || string(problem.Code) != test.code {
				t.Fatalf("problem=%#v err=%v body=%s", problem, err, response.Body.String())
			}
		})
	}
}

func TestItemsLimitBoundariesAndLinkScope(t *testing.T) {
	router := newItemsTestRouter(t)
	for _, limit := range []int{1, 20, 100} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			response := itemsRequest(t, router, "/v1/items?limit="+strconv.Itoa(limit)+"&category=tools")
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			page := decodeItemsPage(t, response)
			if len(page.Items) > limit || page.Total != 6 {
				t.Fatalf("page=%#v", page)
			}
			for _, target := range linkTargets(t, response.Header().Get("Link")) {
				parsed, err := url.Parse(target)
				if err != nil || parsed.IsAbs() || parsed.Path != "/v1/items" ||
					parsed.Query().Get("limit") != strconv.Itoa(limit) || parsed.Query().Get("category") != "tools" {
					t.Fatalf("target=%q parsed=%#v err=%v", target, parsed, err)
				}
			}
		})
	}
}

func TestItemsNegotiatesJSONCBORAndRejectsUnsupportedSuccess(t *testing.T) {
	router := newItemsTestRouter(t)
	for _, mediaType := range []string{"application/json", "application/cbor"} {
		t.Run(mediaType, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/items?limit=1", nil)
			request.Header.Set("Accept", mediaType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != mediaType {
				t.Fatalf(
					"status=%d content-type=%q body=%x",
					response.Code,
					response.Header().Get("Content-Type"),
					response.Body.Bytes(),
				)
			}
			if mediaType == "application/cbor" {
				var page ListData
				if err := cbor.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
					t.Fatalf("decode CBOR page=%#v err=%v", page, err)
				}
			}
		})
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/items", nil)
	request.Header.Set("Accept", "text/plain")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotAcceptable ||
		response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf(
			"status=%d content-type=%q body=%s",
			response.Code,
			response.Header().Get("Content-Type"),
			response.Body.String(),
		)
	}
}
