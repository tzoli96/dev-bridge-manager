// backend/internal/services/jobscraper/manual_fetch_test.go
package jobscraper

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(data)
}

func TestExtractJobPostingPrefersJSONLD(t *testing.T) {
	html := readFixture(t, "manual_jobposting_ldjson.html")
	job, ok := extractJobPosting(html, "https://example.com/job/123")
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if job.Title != "Senior Backend Engineer" {
		t.Errorf("unexpected title: %q", job.Title)
	}
	if job.Company != "Acme Corp" {
		t.Errorf("unexpected company: %q", job.Company)
	}
	if job.Location != "Budapest, Hungary" {
		t.Errorf("unexpected location: %q", job.Location)
	}
	if job.Description != "Build and maintain our core Go services." {
		t.Errorf("unexpected description: %q", job.Description)
	}
	if job.ExternalURL != "https://example.com/job/123" {
		t.Errorf("unexpected url: %q", job.ExternalURL)
	}
}

func TestExtractJobPostingFallsBackToOpenGraph(t *testing.T) {
	html := readFixture(t, "manual_og_only.html")
	job, ok := extractJobPosting(html, "https://nofluffjobs.com/job/example")
	if !ok {
		t.Fatal("expected extraction to succeed via OG fallback")
	}
	if job.Title != "Azure Cloud Infrastructure Engineer" {
		t.Errorf("unexpected title: %q", job.Title)
	}
	if job.Company != "Square One Resources" {
		t.Errorf("unexpected company: %q", job.Company)
	}
	if job.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestExtractJobPostingReturnsNotExtractedWhenNoData(t *testing.T) {
	html := readFixture(t, "manual_no_data.html")
	_, ok := extractJobPosting(html, "https://example.com/nothing")
	if ok {
		t.Fatal("expected extraction to fail (Extracted == false), not an error")
	}
}
