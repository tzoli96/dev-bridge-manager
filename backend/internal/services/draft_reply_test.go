// backend/internal/services/draft_reply_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDraftReplySendsFieldsAndReturnsDraft(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/draft-reply" {
			t.Errorf("expected path /draft-reply, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "Szia! Köszönöm a megkeresésed."})
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	got, err := svc.DraftReply(context.Background(), "email body", "profile context")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Szia! Köszönöm a megkeresésed." {
		t.Fatalf("expected draft text, got %q", got)
	}
	if gotBody["email_content"] != "email body" || gotBody["profile_context"] != "profile context" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestDraftReplyReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.DraftReply(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestDraftReplyReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "d"})
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.DraftReply(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
