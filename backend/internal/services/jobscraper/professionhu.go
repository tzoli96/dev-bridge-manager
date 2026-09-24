// backend/internal/services/jobscraper/professionhu.go
package jobscraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// professionHuListingURLPrefix reproduces the exact query-string shape
// profession.hu uses for its unfiltered, all-category listing: "1," then
// 36 "0," category slots, then the page number. Verified by hand against
// https://www.profession.hu/allasok/1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1
var professionHuListingURLPrefix = "1," + strings.Repeat("0,", 36)

const professionHuMaxPages = 10
const professionHuRequestDelay = 1500 * time.Millisecond

type ProfessionHuScraper struct {
	httpClient *http.Client
	knownURLs  map[string]bool
}

// NewProfessionHuScraper takes the (site, external_url) pairs already
// stored for this site so Scrape can stop paginating once it stops seeing
// new ads - a pagination optimization only. True dedup happens at the
// (site, external_url) DB unique constraint regardless of this map.
func NewProfessionHuScraper(knownURLs map[string]bool) *ProfessionHuScraper {
	return &ProfessionHuScraper{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		knownURLs:  knownURLs,
	}
}

func (s *ProfessionHuScraper) Site() string { return "profession.hu" }

func (s *ProfessionHuScraper) Scrape(ctx context.Context) ([]ScrapedJob, error) {
	allowed, err := robotsAllowed(ctx, s.httpClient, "https://www.profession.hu/allasok")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("profession.hu robots.txt disallows /allasok")
	}

	var jobs []ScrapedJob
	for page := 1; page <= professionHuMaxPages; page++ {
		if page > 1 {
			time.Sleep(professionHuRequestDelay)
		}

		html, err := fetchListingPage(ctx, s.httpClient, page)
		if err != nil {
			return jobs, fmt.Errorf("fetching page %d: %w", page, err)
		}

		pageJobs, err := parseListingPage(html)
		if err != nil {
			return jobs, fmt.Errorf("parsing page %d: %w", page, err)
		}
		if len(pageJobs) == 0 {
			break
		}

		allKnown := true
		for _, job := range pageJobs {
			jobs = append(jobs, job)
			if !s.knownURLs[job.ExternalURL] {
				allKnown = false
			}
		}
		if allKnown {
			break
		}
	}
	return jobs, nil
}

func fetchListingPage(ctx context.Context, client *http.Client, page int) (string, error) {
	trimmedPrefix := strings.TrimSuffix(professionHuListingURLPrefix, ",")
	listingURL := fmt.Sprintf("https://www.profession.hu/allasok/%s,%d", trimmedPrefix, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listingURL, nil)
	if err != nil {
		return "", fmt.Errorf("building listing request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching listing page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading listing page: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("listing page returned %d", resp.StatusCode)
	}
	return string(body), nil
}

// parseListingPage is pure and network-free so it can be unit-tested
// against the saved fixture instead of a live fetch.
func parseListingPage(html string) ([]ScrapedJob, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing listing html: %w", err)
	}

	var jobs []ScrapedJob
	doc.Find("div.dsx-job-card-basic").Each(func(_ int, card *goquery.Selection) {
		titleLink := card.Find(`h2[id$="-title-position"] a`).First()
		title := strings.TrimSpace(titleLink.Text())
		url, _ := titleLink.Attr("href")
		if title == "" || url == "" {
			return
		}

		company := strings.TrimSpace(card.Find(`a[id$="-details-company-name"] span.details-text`).First().Text())
		location := strings.TrimSpace(card.Find(`li[id$="-details-location"] strong.primary-details-location`).First().Text())

		var descParts []string
		card.Find(`div[id$="-details-task-list"] li`).Each(func(_ int, li *goquery.Selection) {
			text := strings.TrimSpace(li.Text())
			if text != "" {
				descParts = append(descParts, text)
			}
		})

		jobs = append(jobs, ScrapedJob{
			ExternalURL: url,
			Title:       title,
			Company:     company,
			Location:    location,
			Description: strings.Join(descParts, "\n"),
		})
	})
	return jobs, nil
}
