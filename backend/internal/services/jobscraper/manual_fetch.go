// backend/internal/services/jobscraper/manual_fetch.go
package jobscraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// placeholderDescriptionPattern matches CDN edge-substitution tokens seen in
// NoFluffJobs' JSON-LD (e.g. "<<ccr:e9c9c5001587,html,1.3KB>>") - their
// description text is injected client-side/at the edge and never appears in
// a plain server-side fetch, so a raw fetch sees the token instead of real
// content.
var placeholderDescriptionPattern = regexp.MustCompile(`^<<ccr:[^>]*>>$`)

// ManualFetchResult is FetchJobFromURL's return value. Extracted is false
// (not an error) when neither JSON-LD nor Open Graph tags gave enough data -
// the caller then falls back to asking the operator for the fields by hand.
type ManualFetchResult struct {
	Job       ScrapedJob
	Extracted bool
}

type jsonLDJobPosting struct {
	Type               string `json:"@type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	HiringOrganization struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
	JobLocation struct {
		Address struct {
			AddressLocality string `json:"addressLocality"`
			AddressRegion   string `json:"addressRegion"`
		} `json:"address"`
	} `json:"jobLocation"`
}

// FetchJobFromURL is a single, explicit, one-off fetch triggered by an
// admin action - no pagination, no repeated scheduling, unlike Scrape.
func FetchJobFromURL(ctx context.Context, rawURL string) (ManualFetchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	allowed, err := robotsAllowed(ctx, client, rawURL)
	if err != nil {
		return ManualFetchResult{}, err
	}
	if !allowed {
		return ManualFetchResult{}, fmt.Errorf("robots.txt disallows fetching %s", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ManualFetchResult{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return ManualFetchResult{}, fmt.Errorf("fetching url: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ManualFetchResult{}, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return ManualFetchResult{}, fmt.Errorf("url returned %d", resp.StatusCode)
	}

	job, extracted := extractJobPosting(string(body), rawURL)
	return ManualFetchResult{Job: job, Extracted: extracted}, nil
}

// extractJobPosting is pure so it can be tested against saved fixtures
// instead of a live fetch - JSON-LD/Open Graph are stable public standards,
// so synthetic fixtures are as reliable as live-fetched ones here.
func extractJobPosting(html, sourceURL string) (ScrapedJob, bool) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ScrapedJob{}, false
	}

	if job, ok := extractFromJSONLD(doc, sourceURL); ok {
		if job.Description == "" || placeholderDescriptionPattern.MatchString(job.Description) {
			job.Description = ogContent(doc, "og:description")
		}
		return job, job.Description != ""
	}
	return extractFromOpenGraph(doc, sourceURL)
}

// jsonLDGraph matches sites (e.g. NoFluffJobs) that wrap multiple schema.org
// entities in a single script block via "@graph" instead of putting a
// JobPosting directly at the top level.
type jsonLDGraph struct {
	Graph []json.RawMessage `json:"@graph"`
}

func extractFromJSONLD(doc *goquery.Document, sourceURL string) (ScrapedJob, bool) {
	var found ScrapedJob
	var ok bool
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		raw := []byte(sel.Text())

		if posting, matched := jobPostingFromJSONLD(raw); matched {
			found, ok = jobFromPosting(posting, sourceURL), true
			return false // stop: found a usable JobPosting
		}

		var graph jsonLDGraph
		if err := json.Unmarshal(raw, &graph); err == nil {
			for _, item := range graph.Graph {
				if posting, matched := jobPostingFromJSONLD(item); matched {
					found, ok = jobFromPosting(posting, sourceURL), true
					return false
				}
			}
		}
		return true // keep looking at other script blocks
	})
	return found, ok
}

func jobPostingFromJSONLD(raw json.RawMessage) (jsonLDJobPosting, bool) {
	var posting jsonLDJobPosting
	if err := json.Unmarshal(raw, &posting); err != nil {
		return jsonLDJobPosting{}, false
	}
	if posting.Type != "JobPosting" || posting.Title == "" {
		return jsonLDJobPosting{}, false
	}
	return posting, true
}

func jobFromPosting(posting jsonLDJobPosting, sourceURL string) ScrapedJob {
	location := strings.Trim(strings.TrimSpace(strings.Join([]string{
		posting.JobLocation.Address.AddressLocality,
		posting.JobLocation.Address.AddressRegion,
	}, ", ")), ", ")
	return ScrapedJob{
		ExternalURL: sourceURL,
		Title:       posting.Title,
		Company:     posting.HiringOrganization.Name,
		Location:    location,
		Description: posting.Description,
	}
}

func extractFromOpenGraph(doc *goquery.Document, sourceURL string) (ScrapedJob, bool) {
	title := ogContent(doc, "og:title")
	description := ogContent(doc, "og:description")
	if title == "" || description == "" {
		return ScrapedJob{}, false
	}
	return ScrapedJob{
		ExternalURL: sourceURL,
		Title:       title,
		Company:     ogContent(doc, "og:site_name"),
		Description: description,
	}, true
}

func ogContent(doc *goquery.Document, property string) string {
	content, _ := doc.Find(fmt.Sprintf(`meta[property="%s"]`, property)).First().Attr("content")
	return strings.TrimSpace(content)
}
