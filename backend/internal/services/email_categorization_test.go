// backend/internal/services/email_categorization_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCategorizeSendsFieldsAndReturnsCategory(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/categorize-email" {
			t.Errorf("expected path /categorize-email, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"category": "szamla"})
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	got, err := svc.Categorize(context.Background(), "Számla #1", "snippet", "billing@x.com", "Billing Co")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != "szamla" {
		t.Fatalf("expected category 'szamla', got %v", got)
	}
	if gotBody["subject"] != "Számla #1" || gotBody["from_address"] != "billing@x.com" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestCategorizeReturnsErrorOnUnknownCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"category": "not-a-real-category"})
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.Categorize(context.Background(), "s", "s", "a", "n")
	if err == nil {
		t.Fatal("expected an error for an unrecognized category, got nil")
	}
}

func TestCategorizeReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.Categorize(context.Background(), "s", "s", "a", "n")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestCategorizeReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"category": "egyeb"})
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.Categorize(context.Background(), "s", "s", "a", "n")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
