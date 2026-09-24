// backend/internal/services/job_application_draft_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobApplicationDraftSendsFieldsAndReturnsDraft(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/job-application-draft" {
			t.Errorf("expected path /job-application-draft, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "Tisztelt Cím!"})
	}))
	defer server.Close()

	svc := &JobApplicationDraftService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	draft, err := svc.DraftApplication(context.Background(), "cv", "skills", "title", "company", "description", "emeld ki a Go tapasztalatot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft != "Tisztelt Cím!" {
		t.Fatalf("unexpected draft: %q", draft)
	}
	if gotBody["instruction"] != "emeld ki a Go tapasztalatot" {
		t.Fatalf("request body missing expected instruction: %+v", gotBody)
	}
}

func TestJobApplicationDraftReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &JobApplicationDraftService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.DraftApplication(context.Background(), "cv", "skills", "title", "company", "description", "")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}
