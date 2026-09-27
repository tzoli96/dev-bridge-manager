// backend/internal/services/email_client_matching_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMatchClientSendsFieldsAndReturnsID(t *testing.T) {
	var gotBody struct {
		Subject     string            `json:"subject"`
		FromAddress string            `json:"from_address"`
		Clients     []ClientCandidate `json:"clients"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/match-email-client" {
			t.Errorf("expected path /match-email-client, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"client_id": 2})
	}))
	defer server.Close()

	svc := &EmailClientMatchingService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	candidates := []ClientCandidate{{ID: 1, Name: "Acme", Email: "a@acme.com"}, {ID: 2, Name: "Beta", Email: "b@beta.com"}}
	got, err := svc.MatchClient(context.Background(), "subj", "snip", "jane@beta.com", "Jane", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != 2 {
		t.Fatalf("expected client_id 2, got %v", got)
	}
	if gotBody.FromAddress != "jane@beta.com" || len(gotBody.Clients) != 2 {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestMatchClientReturnsNilWhenAIReportsNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"client_id": nil})
	}))
	defer server.Close()

	svc := &EmailClientMatchingService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	got, err := svc.MatchClient(context.Background(), "s", "s", "a", "n", []ClientCandidate{{ID: 1, Name: "Acme", Email: "a@acme.com"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil client_id, got %v", *got)
	}
}

func TestMatchClientReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &EmailClientMatchingService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.MatchClient(context.Background(), "s", "s", "a", "n", []ClientCandidate{{ID: 1}})
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestMatchClientReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"client_id": nil})
	}))
	defer server.Close()

	svc := &EmailClientMatchingService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.MatchClient(context.Background(), "s", "s", "a", "n", []ClientCandidate{{ID: 1}})
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
