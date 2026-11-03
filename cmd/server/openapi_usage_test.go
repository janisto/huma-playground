package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIOperationsHaveNativeUsageDocumentation(t *testing.T) {
	router := testRouter(t, testConfig(t))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("OpenAPI status = %d", response.Code)
	}
	var document struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	count := 0
	for path, methods := range document.Paths {
		for method, operation := range methods {
			count++
			if strings.TrimSpace(operation.Summary) == "" || strings.TrimSpace(operation.Description) == "" {
				t.Errorf("%s %s (%s) lacks native summary or description", method, path, operation.OperationID)
			}
		}
	}
	if count != 14 {
		t.Fatalf("documented operations = %d, want 14", count)
	}
}
