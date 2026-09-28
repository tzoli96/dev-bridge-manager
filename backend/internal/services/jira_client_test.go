// backend/internal/services/jira_client_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestAdfToPlainText_LegacyString(t *testing.T) {
	raw := json.RawMessage(`"Plain legacy description"`)
	got := adfToPlainText(raw)
	if got != "Plain legacy description" {
		t.Errorf("got %q, want %q", got, "Plain legacy description")
	}
}

func TestAdfToPlainText_ADFDocument(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "doc",
		"content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "First line"}]},
			{"type": "paragraph", "content": [{"type": "text", "text": "Second line"}]}
		]
	}`)
	got := adfToPlainText(raw)
	want := "First line\nSecond line"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToPlainText_NilOrEmpty(t *testing.T) {
	if got := adfToPlainText(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
	if got := adfToPlainText(json.RawMessage("null")); got != "" {
		t.Errorf("null: got %q, want empty", got)
	}
	if got := adfToPlainText(json.RawMessage("")); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}
}

func TestRealJiraClient_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "me@example.com" || pass != "token123" {
			t.Errorf("unexpected basic auth: %s/%s (ok=%v)", user, pass, ok)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"abc"}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "me@example.com", "token123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRealJiraClient_TestConnection_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "me@example.com", "wrong-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRealJiraClient_SearchAssignedIssues_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"issues": [
				{"key": "PROJ-1", "fields": {"summary": "First issue", "description": "Plain text", "status": {"name": "To Do"}}}
			],
			"isLast": true
		}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	integration := models.JiraBoardIntegration{BaseURL: server.URL, Email: "me@example.com", APIToken: "token123", ProjectKey: "PROJ"}
	issues, err := client.SearchAssignedIssues(context.Background(), integration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Key != "PROJ-1" || issues[0].Summary != "First issue" || issues[0].Description != "Plain text" || issues[0].StatusName != "To Do" {
		t.Errorf("unexpected issue: %+v", issues[0])
	}
}

func TestRealJiraClient_SearchAssignedIssues_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("nextPageToken") == "" {
			w.Write([]byte(`{
				"issues": [{"key": "PROJ-1", "fields": {"summary": "First", "status": {"name": "To Do"}}}],
				"nextPageToken": "page2",
				"isLast": false
			}`))
		} else {
			w.Write([]byte(`{
				"issues": [{"key": "PROJ-2", "fields": {"summary": "Second", "status": {"name": "In Progress"}}}],
				"isLast": true
			}`))
		}
	}))
	defer server.Close()

	client := NewRealJiraClient()
	integration := models.JiraBoardIntegration{BaseURL: server.URL, Email: "me@example.com", APIToken: "token123", ProjectKey: "PROJ"}
	issues, err := client.SearchAssignedIssues(context.Background(), integration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 requests, got %d", callCount)
	}
	if len(issues) != 2 || issues[0].Key != "PROJ-1" || issues[1].Key != "PROJ-2" {
		t.Errorf("unexpected issues: %+v", issues)
	}
}

func TestRealJiraClient_SearchAssignedIssues_ParsesADFDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"issues": [{
				"key": "PROJ-3",
				"fields": {
					"summary": "ADF issue",
					"description": {"type": "doc", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Rich description"}]}]},
					"status": {"name": "To Do"}
				}
			}],
			"isLast": true
		}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	integration := models.JiraBoardIntegration{BaseURL: server.URL, Email: "me@example.com", APIToken: "token123", ProjectKey: "PROJ"}
	issues, err := client.SearchAssignedIssues(context.Background(), integration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(issues) != 1 || issues[0].Description != "Rich description" {
		t.Errorf("unexpected issues: %+v", issues)
	}
}
