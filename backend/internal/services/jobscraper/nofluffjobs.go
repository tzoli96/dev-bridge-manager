// backend/internal/services/jobscraper/nofluffjobs.go
package jobscraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// noFluffJobsCategorySlugs are the IT-relevant category landing pages
// listed in nofluffjobs.com's own navigation (see
// https://nofluffjobs.com/hu). Non-IT categories the same nav exposes
// (marketing, sales, hr, law, finance, mechanics, ...) are deliberately
// excluded, same reasoning as professionHuCategorySlug: an unfiltered feed
// drowns real matches in noise.
var noFluffJobsCategorySlugs = []string{
	"backend", "frontend", "fullstack", "devops", "data", "mobile",
	"security", "testing", "embedded", "artificial-intelligence",
	"automation", "architecture", "sys-administrator", "business-intelligence",
	"support", "agile", "game-dev", "ux", "pm", "product-management",
}

const noFluffJobsRequestDelay = 1500 * time.Millisecond

// NoFluffJobsScraper works around a hard limit discovered while
// investigating this site: it has no sitemap, its /api/ search endpoint is
// disallowed by robots.txt, and its server-rendered category pages
// (/hu/{category}) always render page 1 of that category regardless of any
// "page" query param - true pagination only happens client-side against the
// disallowed API. So instead of paginating one feed (like ProfessionHuScraper),
// this walks every IT category's page 1 (each page embeds its results as
// JSON in a <script id="serverApp-state"> tag - see parseCategoryPage),
// dedupes the resulting job URLs, keeps the maxJobs freshest, and fetches
// each one via the existing manual-add extraction path (FetchJobFromURL)
// to get its full description - category pages don't include descriptions.
type NoFluffJobsScraper struct {
	httpClient *http.Client
	maxJobs    int
}

func NewNoFluffJobsScraper(maxJobs int) *NoFluffJobsScraper {
	return &NoFluffJobsScraper{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		maxJobs:    maxJobs,
	}
}

func (s *NoFluffJobsScraper) Site() string { return "nofluffjobs.com" }

func (s *NoFluffJobsScraper) Scrape(ctx context.Context) ([]ScrapedJob, error) {
	allowed, err := robotsAllowed(ctx, s.httpClient, "https://nofluffjobs.com/hu")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("nofluffjobs.com robots.txt disallows /hu")
	}

	var candidates []jobCandidate
	for i, slug := range noFluffJobsCategorySlugs {
		if i > 0 {
			time.Sleep(noFluffJobsRequestDelay)
		}
		html, err := fetchCategoryPage(ctx, s.httpClient, slug)
		if err != nil {
			continue // one bad category shouldn't abort the whole run
		}
		postings, err := parseCategoryPage(html)
		if err != nil {
			continue
		}
		for _, p := range postings {
			candidates = append(candidates, jobCandidate{
				url:     "https://nofluffjobs.com/hu/job/" + p.URL,
				renewed: p.Renewed,
			})
		}
	}

	candidates = dedupeAndSortCandidates(candidates)
	if len(candidates) > s.maxJobs {
		candidates = candidates[:s.maxJobs]
	}

	var jobs []ScrapedJob
	for i, cand := range candidates {
		if i > 0 {
			time.Sleep(noFluffJobsRequestDelay)
		}
		result, err := FetchJobFromURL(ctx, cand.url)
		if err != nil || !result.Extracted {
			continue // same per-item tolerance as RunScrape
		}
		jobs = append(jobs, result.Job)
	}
	return jobs, nil
}

func fetchCategoryPage(ctx context.Context, client *http.Client, slug string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://nofluffjobs.com/hu/"+slug, nil)
	if err != nil {
		return "", fmt.Errorf("building category request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching category page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading category page: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("category page returned %d", resp.StatusCode)
	}
	return string(body), nil
}

// categoryPosting is the subset of fields nofluffjobs.com embeds per
// listing in its category pages' serverApp-state JSON blob.
type categoryPosting struct {
	Title   string `json:"title"`
	Company string `json:"name"`
	URL     string `json:"url"`
	Renewed int64  `json:"renewed"`
	Posted  int64  `json:"posted"`
}

// parseCategoryPage is pure and network-free so it can be unit-tested
// against a saved fixture instead of a live fetch, matching
// parseListingPage's pattern in professionhu.go.
func parseCategoryPage(html string) ([]categoryPosting, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing category html: %w", err)
	}

	stateText := doc.Find(`script#serverApp-state`).First().Text()
	if stateText == "" {
		return nil, fmt.Errorf("serverApp-state script not found")
	}

	var state struct {
		StoreKey struct {
			SearchResponse struct {
				Postings []categoryPosting `json:"postings"`
			} `json:"searchResponse"`
		} `json:"STORE_KEY"`
	}
	if err := json.Unmarshal([]byte(stateText), &state); err != nil {
		return nil, fmt.Errorf("parsing serverApp-state json: %w", err)
	}
	return state.StoreKey.SearchResponse.Postings, nil
}

// jobCandidate is a discovered job URL plus enough to rank it - category
// pages carry no description, so the URL is all that's needed before the
// per-job fetch in Scrape.
type jobCandidate struct {
	url     string
	renewed int64
}

// dedupeAndSortCandidates collapses URLs seen in multiple categories and
// orders by renewed (freshest first), so a maxJobs cutoff keeps the most
// current postings rather than an arbitrary category-order prefix.
func dedupeAndSortCandidates(candidates []jobCandidate) []jobCandidate {
	seen := make(map[string]bool, len(candidates))
	var unique []jobCandidate
	for _, c := range candidates {
		if seen[c.url] {
			continue
		}
		seen[c.url] = true
		unique = append(unique, c)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].renewed > unique[j].renewed
	})
	return unique
}
