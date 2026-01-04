package items

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	applog "github.com/janisto/huma-playground/internal/platform/logging"
	appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
	"github.com/janisto/huma-playground/internal/platform/pagination"
	"github.com/janisto/huma-playground/internal/platform/respond"
)

func newTestRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(
		appmiddleware.RequestID(),
		chimiddleware.RealIP,
		applog.RequestLogger(),
		respond.Recoverer(),
	)
	api := humachi.New(router, huma.DefaultConfig("ItemsTest", "test"))
	Register(api)
	return router
}

func TestListFirstPage(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-first-page")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 20 {
		t.Errorf("expected 20 items (default limit), got %d", len(data.Items))
	}
	if data.Total != 30 {
		t.Errorf("expected total 30, got %d", data.Total)
	}
	if data.Items[0].ID != "item-001" {
		t.Errorf("expected first item item-001, got %s", data.Items[0].ID)
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, `rel="next"`) {
		t.Error("expected Link header with rel=next")
	}
	if strings.Contains(linkHeader, `rel="prev"`) {
		t.Error("first page should not have rel=prev")
	}
}

func TestListWithLimit(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=5", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-limit-5")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 5 {
		t.Errorf("expected 5 items, got %d", len(data.Items))
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, `rel="next"`) {
		t.Error("expected Link header with rel=next")
	}
}

func TestListMiddlePage(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "item", Value: "item-010"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor+"&limit=5", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-middle-page")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 5 {
		t.Errorf("expected 5 items, got %d", len(data.Items))
	}
	if data.Items[0].ID != "item-011" {
		t.Errorf("expected first item item-011, got %s", data.Items[0].ID)
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, `rel="next"`) {
		t.Error("middle page should have rel=next")
	}
	if !strings.Contains(linkHeader, `rel="prev"`) {
		t.Error("middle page should have rel=prev")
	}
}

func TestListLastPage(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "item", Value: "item-025"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor+"&limit=10", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-last-page")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 5 {
		t.Errorf("expected 5 items (remaining), got %d", len(data.Items))
	}
	if data.Items[0].ID != "item-026" {
		t.Errorf("expected first item item-026, got %s", data.Items[0].ID)
	}

	linkHeader := resp.Header().Get("Link")
	if strings.Contains(linkHeader, `rel="next"`) {
		t.Error("last page should not have rel=next")
	}
	if !strings.Contains(linkHeader, `rel="prev"`) {
		t.Error("last page should have rel=prev")
	}
}

func TestListWithCategoryFilter(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?category=tools", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-category-filter")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	for _, item := range data.Items {
		if item.Category != "tools" {
			t.Errorf("expected category tools, got %s for item %s", item.Category, item.ID)
		}
	}

	if data.Total == 0 {
		t.Error("expected at least one tool item")
	}
}

func TestListCategoryPreservedInLink(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?category=electronics&limit=5", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-category-link")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, "category=electronics") {
		t.Error("category filter not preserved in Link header")
	}
}

func TestListInvalidCursor(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?cursor=invalid!!!", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-invalid-cursor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal problem: %v", err)
	}

	if problem.Status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", problem.Status)
	}
}

func TestListCursorTypeMismatch(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "wrongtype", Value: "item-001"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor, nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-cursor-mismatch")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal problem: %v", err)
	}

	if !strings.Contains(problem.Detail, "cursor type mismatch") {
		t.Errorf("expected cursor type mismatch error, got %s", problem.Detail)
	}
}

func TestListCursorUnknownItem(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "item", Value: "nonexistent"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor, nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-cursor-unknown")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal problem: %v", err)
	}

	if !strings.Contains(problem.Detail, "unknown item") {
		t.Errorf("expected unknown item error, got %s", problem.Detail)
	}
}

func TestListCBOR(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=3", nil)
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set(chimiddleware.RequestIDHeader, "items-cbor")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	if ct := resp.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Errorf("expected application/cbor, got %s", ct)
	}

	var data ListData
	if err := cbor.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("cbor unmarshal: %v", err)
	}

	if len(data.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(data.Items))
	}
}

func TestListInvalidCategory(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?category=nonexistent", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-invalid-category")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.Code)
	}

	var problem huma.ErrorModel
	if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
		t.Fatalf("json unmarshal problem: %v", err)
	}

	if problem.Status != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", problem.Status)
	}
}

func TestListPaginationRoundTrip(t *testing.T) {
	router := newTestRouter()
	collectedIDs := make(map[string]bool)
	limit := 7

	req := httptest.NewRequest(http.MethodGet, "/items?limit=7", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-roundtrip-1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("first page: expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	for _, item := range data.Items {
		collectedIDs[item.ID] = true
	}

	pageCount := 1
	linkHeader := resp.Header().Get("Link")

	for strings.Contains(linkHeader, `rel="next"`) && pageCount < 10 {
		nextURL := extractLinkURL(linkHeader, "next")
		if nextURL == "" {
			t.Fatal("could not extract next URL from Link header")
		}

		req = httptest.NewRequest(http.MethodGet, nextURL, nil)
		req.Header.Set(chimiddleware.RequestIDHeader, "items-roundtrip-"+string(rune('0'+pageCount+1)))
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("page %d: expected 200, got %d", pageCount+1, resp.Code)
		}

		if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
			t.Fatalf("page %d json unmarshal: %v", pageCount+1, err)
		}

		for _, item := range data.Items {
			if collectedIDs[item.ID] {
				t.Errorf("duplicate item %s on page %d", item.ID, pageCount+1)
			}
			collectedIDs[item.ID] = true
		}

		linkHeader = resp.Header().Get("Link")
		pageCount++
	}

	expectedPages := (30 + limit - 1) / limit
	if pageCount != expectedPages {
		t.Errorf("expected %d pages, got %d", expectedPages, pageCount)
	}

	if len(collectedIDs) != 30 {
		t.Errorf("expected 30 unique items, got %d", len(collectedIDs))
	}
}

func TestListValidateLimitRange(t *testing.T) {
	router := newTestRouter()

	tests := []struct {
		name     string
		limit    string
		wantCode int
	}{
		{"limit-too-high", "150", http.StatusUnprocessableEntity},
		{"limit-zero", "0", http.StatusUnprocessableEntity},
		{"limit-negative", "-5", http.StatusUnprocessableEntity},
		{"limit-valid-min", "1", http.StatusOK},
		{"limit-valid-max", "100", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/items?limit="+tc.limit, nil)
			req.Header.Set(chimiddleware.RequestIDHeader, "items-limit-"+tc.name)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantCode {
				t.Errorf("expected %d, got %d", tc.wantCode, resp.Code)
			}
		})
	}
}

func TestListCursorAtFirstItem(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "item", Value: "item-001"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor+"&limit=5", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-cursor-first")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if data.Items[0].ID != "item-002" {
		t.Errorf("expected first item item-002 (after cursor), got %s", data.Items[0].ID)
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, `rel="prev"`) {
		t.Error("should have prev link even after first item")
	}
}

func TestListCursorAtLastItem(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "item", Value: "item-030"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor+"&limit=5", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-cursor-last")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 0 {
		t.Errorf("expected 0 items after last item, got %d", len(data.Items))
	}

	linkHeader := resp.Header().Get("Link")
	if strings.Contains(linkHeader, `rel="next"`) {
		t.Error("should not have next link after last item")
	}
	if !strings.Contains(linkHeader, `rel="prev"`) {
		t.Error("should have prev link")
	}
}

func TestListSingleItemPage(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=1", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-single")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(data.Items))
	}
	if data.Items[0].ID != "item-001" {
		t.Errorf("expected item-001, got %s", data.Items[0].ID)
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, `rel="next"`) {
		t.Error("should have next link")
	}
}

func TestListLimitPreservedInLink(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=7", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-limit-link")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, "limit=7") {
		t.Errorf("limit not preserved in Link header: %s", linkHeader)
	}
}

func TestListBackwardsNavigation(t *testing.T) {
	router := newTestRouter()

	cursor := pagination.Cursor{Type: "item", Value: "item-005"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/items?cursor="+cursor+"&limit=5", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-backwards")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	linkHeader := resp.Header().Get("Link")
	prevURL := extractLinkURL(linkHeader, "prev")
	if prevURL == "" {
		t.Fatal("expected prev link")
	}

	req = httptest.NewRequest(http.MethodGet, prevURL, nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-backwards-prev")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("prev page: expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if data.Items[0].ID != "item-001" {
		t.Errorf("expected first page to start with item-001, got %s", data.Items[0].ID)
	}
}

func TestListExactPageBoundary(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?limit=30", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-exact-boundary")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 30 {
		t.Errorf("expected 30 items, got %d", len(data.Items))
	}

	linkHeader := resp.Header().Get("Link")
	if strings.Contains(linkHeader, `rel="next"`) {
		t.Error("should not have next when all items fit in one page")
	}
	if strings.Contains(linkHeader, `rel="prev"`) {
		t.Error("first page should not have prev")
	}
}

func TestListCategoryFilterWithPagination(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?category=electronics&limit=3", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-category-pagination")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if len(data.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(data.Items))
	}

	for _, item := range data.Items {
		if item.Category != "electronics" {
			t.Errorf("expected electronics category, got %s", item.Category)
		}
	}

	linkHeader := resp.Header().Get("Link")
	if !strings.Contains(linkHeader, "category=electronics") {
		t.Error("category should be preserved in next link")
	}
	if !strings.Contains(linkHeader, `rel="next"`) {
		t.Error("should have next link for filtered results")
	}
}

func TestListValidCategory(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?category=electronics", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-valid-category")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if data.Total == 0 {
		t.Error("expected items for electronics category")
	}

	for _, item := range data.Items {
		if item.Category != "electronics" {
			t.Errorf("expected electronics category, got %s", item.Category)
		}
	}
}

func TestListValidationErrors422(t *testing.T) {
	router := newTestRouter()

	tests := []struct {
		name       string
		query      string
		wantField  string
		wantStatus int
	}{
		{
			name:       "category-uppercase",
			query:      "?category=ELECTRONICS",
			wantField:  "category",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "category-mixed-case",
			query:      "?category=Electronics",
			wantField:  "category",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "category-invalid-value",
			query:      "?category=furniture",
			wantField:  "category",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "limit-exceeds-max",
			query:      "?limit=101",
			wantField:  "limit",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "limit-below-min",
			query:      "?limit=0",
			wantField:  "limit",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "limit-negative",
			query:      "?limit=-10",
			wantField:  "limit",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "limit-non-numeric",
			query:      "?limit=abc",
			wantField:  "limit",
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "limit-float",
			query:      "?limit=5.5",
			wantField:  "limit",
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/items"+tc.query, nil)
			req.Header.Set(chimiddleware.RequestIDHeader, "items-422-"+tc.name)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, resp.Code)
			}

			ct := resp.Header().Get("Content-Type")
			if ct != "application/problem+json" {
				t.Errorf("expected application/problem+json, got %s", ct)
			}

			var problem huma.ErrorModel
			if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
				t.Fatalf("json unmarshal problem: %v", err)
			}

			if problem.Status != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, problem.Status)
			}

			foundField := false
			for _, e := range problem.Errors {
				if strings.Contains(e.Location, tc.wantField) {
					foundField = true
					break
				}
			}
			if !foundField {
				t.Errorf("expected error for field %q, errors: %+v", tc.wantField, problem.Errors)
			}
		})
	}
}

func TestListEmptyCategoryReturnsAll(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?category=", nil)
	req.Header.Set(chimiddleware.RequestIDHeader, "items-empty-category")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var data ListData
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if data.Total != 30 {
		t.Errorf("empty category should return all items, got %d", data.Total)
	}
}

func TestListCursorErrors400(t *testing.T) {
	router := newTestRouter()

	tests := []struct {
		name        string
		cursor      string
		wantMessage string
	}{
		{
			name:        "invalid-base64",
			cursor:      "!!!notbase64!!!",
			wantMessage: "invalid cursor",
		},
		{
			name:        "wrong-cursor-type",
			cursor:      pagination.Cursor{Type: "order", Value: "item-001"}.Encode(),
			wantMessage: "cursor type mismatch",
		},
		{
			name:        "unknown-item-id",
			cursor:      pagination.Cursor{Type: "item", Value: "item-999"}.Encode(),
			wantMessage: "unknown item",
		},
		{
			name:        "malformed-cursor-no-separator",
			cursor:      "dGVzdA",
			wantMessage: "invalid cursor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/items?cursor="+tc.cursor, nil)
			req.Header.Set(chimiddleware.RequestIDHeader, "items-400-"+tc.name)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.Code)
			}

			ct := resp.Header().Get("Content-Type")
			if ct != "application/problem+json" {
				t.Errorf("expected application/problem+json, got %s", ct)
			}

			var problem huma.ErrorModel
			if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
				t.Fatalf("json unmarshal problem: %v", err)
			}

			if problem.Status != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", problem.Status)
			}

			if !strings.Contains(strings.ToLower(problem.Detail), tc.wantMessage) {
				t.Errorf("expected detail to contain %q, got %q", tc.wantMessage, problem.Detail)
			}
		})
	}
}

func TestListAllValidCategories(t *testing.T) {
	router := newTestRouter()

	categories := []string{"electronics", "tools", "accessories", "robotics", "power", "components"}

	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/items?category="+category, nil)
			req.Header.Set(chimiddleware.RequestIDHeader, "items-category-"+category)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200 for category %s, got %d", category, resp.Code)
			}

			var data ListData
			if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
				t.Fatalf("json unmarshal: %v", err)
			}

			if data.Total == 0 {
				t.Errorf("expected items for category %s", category)
			}

			for _, item := range data.Items {
				if item.Category != category {
					t.Errorf("expected category %s, got %s", category, item.Category)
				}
			}
		})
	}
}

func extractLinkURL(linkHeader, rel string) string {
	for part := range strings.SplitSeq(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="`+rel+`"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start >= 0 && end > start {
				return part[start+1 : end]
			}
		}
	}
	return ""
}
