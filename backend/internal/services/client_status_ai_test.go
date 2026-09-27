// backend/internal/services/client_status_ai_test.go
package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDraftClientStatusEmailSendsFieldsAndReturnsSubjectAndBody(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/client-status-email" {
			t.Errorf("expected path /client-status-email, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"subject": "Heti státusz", "body": "Szia!"})
	}))
	defer server.Close()

	svc := &ClientStatusAIService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	subject, body, err := svc.DraftClientStatusEmail(context.Background(), ClientStatusFacts{
		ClientName:      "Acme Kft.",
		PeriodStart:     "2026-09-21",
		PeriodEnd:       "2026-09-27",
		CompletedTasks:  []string{"Feladat A"},
		HoursLogged:     5,
		InvoicesCreated: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subject != "Heti státusz" || body != "Szia!" {
		t.Fatalf("expected drafted subject/body, got %q / %q", subject, body)
	}
	if gotBody["client_name"] != "Acme Kft." || gotBody["period_start"] != "2026-09-21" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
	gotTasks, ok := gotBody["completed_tasks"].([]interface{})
	if !ok || len(gotTasks) != 1 || gotTasks[0] != "Feladat A" {
		t.Fatalf("request body missing expected completed_tasks: %+v", gotBody)
	}
}

func TestDraftClientStatusEmailMarshalsNilCompletedTasksAsEmptyArray(t *testing.T) {
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"subject": "s", "body": "b"})
	}))
	defer server.Close()

	svc := &ClientStatusAIService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	if _, _, err := svc.DraftClientStatusEmail(context.Background(), ClientStatusFacts{ClientName: "Acme Kft."}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(rawBody), `"completed_tasks":[]`) {
		t.Fatalf("expected completed_tasks to marshal as [], got raw body: %s", rawBody)
	}
}

func TestDraftClientStatusEmailReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &ClientStatusAIService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, _, err := svc.DraftClientStatusEmail(context.Background(), ClientStatusFacts{ClientName: "Acme Kft."})
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}
