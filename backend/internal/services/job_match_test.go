// backend/internal/services/job_match_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobMatchSendsFieldsAndReturnsScore(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/job-match" {
			t.Errorf("expected path /job-match, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"score": 85, "reasoning": "Erős egyezés."})
	}))
	defer server.Close()

	svc := &JobMatchService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	score, reasoning, err := svc.MatchJob(context.Background(), "cv", "skills", "prefs", "title", "company", "location", "description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 || reasoning != "Erős egyezés." {
		t.Fatalf("unexpected result: score=%d reasoning=%q", score, reasoning)
	}
	if gotBody["job_title"] != "title" || gotBody["company"] != "company" || gotBody["location"] != "location" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestJobMatchReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &JobMatchService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, _, err := svc.MatchJob(context.Background(), "cv", "skills", "", "title", "company", "location", "description")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}
