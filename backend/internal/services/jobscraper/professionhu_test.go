// backend/internal/services/jobscraper/professionhu_test.go
package jobscraper

import (
	"os"
	"strings"
	"testing"
)

func TestParseListingPageExtractsRealJobs(t *testing.T) {
	html, err := os.ReadFile("testdata/professionhu_sample.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	jobs, err := parseListingPage(string(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	first := jobs[0]
	if first.Title != "Motorkerékpár szerelő" {
		t.Errorf("unexpected title: %q", first.Title)
	}
	if first.ExternalURL != "https://www.profession.hu/allas/motorkerekpar-szerelo-meteor-motortech-kft-budapest-3010682" {
		t.Errorf("unexpected url: %q", first.ExternalURL)
	}
	if first.Company != "Meteor Motortech Kft." {
		t.Errorf("unexpected company: %q", first.Company)
	}
	if first.Location != "Budapest IX.kerület" {
		t.Errorf("unexpected location: %q", first.Location)
	}
	if !strings.Contains(first.Description, "Brit motorkerékpárok szakszerű karbantartása") {
		t.Errorf("expected description to include task bullet, got: %q", first.Description)
	}

	second := jobs[1]
	if second.Title != "ANYAGKEZELŐ / ALAPANYAG KOMISSIÓZÓ" {
		t.Errorf("unexpected title: %q", second.Title)
	}
	if second.Company != "CELLCOMP KFT." {
		t.Errorf("unexpected company: %q", second.Company)
	}
	if second.Location != "Celldömölk" {
		t.Errorf("unexpected location: %q", second.Location)
	}
}

func TestParseListingPageReturnsEmptyForNoCards(t *testing.T) {
	jobs, err := parseListingPage("<html><body>no cards here</body></html>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d", len(jobs))
	}
}
