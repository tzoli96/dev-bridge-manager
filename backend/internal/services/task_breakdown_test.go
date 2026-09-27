// backend/internal/services/task_breakdown_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBreakdownIntoTasksSendsFieldsAndReturnsGroups(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/task-breakdown" {
			t.Errorf("expected path /task-breakdown, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"groups": []map[string]interface{}{
				{
					"title":       "Válaszolj az ügyfélnek",
					"description": "Az ügyfél árajánlatot kér.",
					"subtasks": []map[string]interface{}{
						{"title": "Csatold az árajánlatot", "description": "Küldd el a PDF-et."},
					},
				},
			},
		})
	}))
	defer server.Close()

	svc := &TaskBreakdownService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	groups, err := svc.BreakdownIntoTasks(context.Background(), "Árajánlat", "email body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 || groups[0].Title != "Válaszolj az ügyfélnek" {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	if len(groups[0].Subtasks) != 1 || groups[0].Subtasks[0].Title != "Csatold az árajánlatot" {
		t.Fatalf("expected subtask, got: %+v", groups[0].Subtasks)
	}
	if gotBody["email_content"] != "email body" || gotBody["subject"] != "Árajánlat" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestBreakdownIntoTasksReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &TaskBreakdownService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.BreakdownIntoTasks(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestBreakdownIntoTasksReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"groups": []interface{}{}})
	}))
	defer server.Close()

	svc := &TaskBreakdownService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.BreakdownIntoTasks(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
