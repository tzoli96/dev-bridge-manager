// backend/internal/services/billingo_service_test.go
package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoRequestSetsAuthHeaderAndParsesBody(t *testing.T) {
	var gotKey, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-KEY")
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer server.Close()

	svc := NewBillingoService()
	body, err := svc.doRequest(server.URL, "test-key", http.MethodPost, "/partners", map[string]string{"name": "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "test-key" {
		t.Fatalf("expected X-API-KEY header 'test-key', got %q", gotKey)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}

	var parsed map[string]int
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if parsed["id"] != 42 {
		t.Fatalf("expected id 42, got %d", parsed["id"])
	}
}

func TestDoRequestReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid api key"}`))
	}))
	defer server.Close()

	svc := NewBillingoService()
	_, err := svc.doRequest(server.URL, "bad-key", http.MethodGet, "/partners", nil)
	if err == nil {
		t.Fatal("expected an error for a 401 response, got nil")
	}
}
