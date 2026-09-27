// backend/internal/services/jobscraper/nofluffjobs_test.go
package jobscraper

import (
	"os"
	"testing"
)

func TestParseCategoryPageExtractsPostings(t *testing.T) {
	html, err := os.ReadFile("testdata/nofluffjobs_category_sample.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	postings, err := parseCategoryPage(string(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(postings))
	}

	first := postings[0]
	if first.Title != "Senior Backend Developer" {
		t.Errorf("unexpected title: %q", first.Title)
	}
	if first.Company != "Deutsche Telekom IT Solutions Hungary" {
		t.Errorf("unexpected company: %q", first.Company)
	}
	if first.URL != "senior-backend-developer-deutsche-telekom-it-solutions-hungary-budapest" {
		t.Errorf("unexpected url slug: %q", first.URL)
	}
	if first.Renewed != 1790258909846 {
		t.Errorf("unexpected renewed timestamp: %d", first.Renewed)
	}
}

func TestParseCategoryPageReturnsErrorWhenStateMissing(t *testing.T) {
	_, err := parseCategoryPage("<html><body>no state here</body></html>")
	if err == nil {
		t.Fatal("expected an error when serverApp-state script is missing")
	}
}

func TestDedupeAndSortCandidatesByFreshness(t *testing.T) {
	candidates := []jobCandidate{
		{url: "https://nofluffjobs.com/hu/job/older", renewed: 100},
		{url: "https://nofluffjobs.com/hu/job/newer", renewed: 300},
		{url: "https://nofluffjobs.com/hu/job/newer", renewed: 300}, // duplicate, seen in another category
		{url: "https://nofluffjobs.com/hu/job/middle", renewed: 200},
	}

	result := dedupeAndSortCandidates(candidates)

	if len(result) != 3 {
		t.Fatalf("expected 3 unique candidates, got %d", len(result))
	}
	if result[0].url != "https://nofluffjobs.com/hu/job/newer" {
		t.Errorf("expected freshest first, got %q", result[0].url)
	}
	if result[1].url != "https://nofluffjobs.com/hu/job/middle" {
		t.Errorf("expected middle second, got %q", result[1].url)
	}
	if result[2].url != "https://nofluffjobs.com/hu/job/older" {
		t.Errorf("expected oldest last, got %q", result[2].url)
	}
}
