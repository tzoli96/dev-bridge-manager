// backend/internal/services/jobscraper/robots_test.go
package jobscraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDisallowedPathsOnlyWildcardBlock(t *testing.T) {
	body := strings.NewReader(`User-agent: Googlebot
Disallow: /only-for-google

User-agent: *
Disallow: /api/
Disallow: /admin/
`)
	got := parseDisallowedPaths(body)
	want := []string{"/api/", "/admin/"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestParseDisallowedPathsIgnoresComments(t *testing.T) {
	body := strings.NewReader(`# comment
User-agent: *
# another comment
Disallow: /private/
`)
	got := parseDisallowedPaths(body)
	if len(got) != 1 || got[0] != "/private/" {
		t.Fatalf("expected [/private/], got %v", got)
	}
}

func TestRobotsAllowedRespectsDisallow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := server.Client()

	allowed, err := robotsAllowed(context.Background(), client, server.URL+"/private/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected disallowed path to be rejected")
	}

	allowed, err = robotsAllowed(context.Background(), client, server.URL+"/public/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected non-disallowed path to be allowed")
	}
}

func TestRobotsAllowedTreatsMissingRobotsTxtAsAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	allowed, err := robotsAllowed(context.Background(), server.Client(), server.URL+"/anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected missing robots.txt to be treated as allowed")
	}
}
