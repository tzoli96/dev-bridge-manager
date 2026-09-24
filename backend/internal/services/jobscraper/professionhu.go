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

// professionHuCategorySlug scopes scraping to profession.hu's "IT
// (Programozás, fejlesztés)" category instead of every category on the
// site. The unfiltered listing (every category) drowned real developer
// postings in a huge volume of unrelated jobs (retail, logistics, HR,
// ...), so AI-matching alone rarely surfaced a genuine fit - this trades
// the original any-category-then-AI-filter design for a category that
// actually matches the profile, per explicit request. Verified by hand:
// https://www.profession.hu/allasok/it-programozas-fejlesztes/1,10 (page 1),
// https://www.profession.hu/allasok/it-programozas-fejlesztes/2,10 (page 2).
const professionHuCategorySlug = "it-programozas-fejlesztes"

const professionHuRequestDelay = 1500 * time.Millisecond

type ProfessionHuScraper struct {
	httpClient *http.Client
	maxPages   int
}

// NewProfessionHuScraper's maxPages is the only paging stop besides an
// empty page - it used to also stop once a page was fully already-known,
// but that assumed a single stable feed ordering, which broke the moment
// scraping moved from the unfiltered listing to a specific category (see
// professionHuCategorySlug): a category page can coincidentally look
// "fully known" from the old unfiltered scrape while the rest of the
// category was never seen. The category is small enough (~60 pages) that
// walking up to maxPages every run is cheap, so that's now the only cap.
// The caller picks maxPages based on how long the run is allowed to take
// (see job_search_handler.go's ScanNow and job_scraping_scheduler.go).
func NewProfessionHuScraper(maxPages int) *ProfessionHuScraper {
	return &ProfessionHuScraper{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		maxPages:   maxPages,
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
	for page := 1; page <= s.maxPages; page++ {
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

		jobs = append(jobs, pageJobs...)
	}
	return jobs, nil
}

func fetchListingPage(ctx context.Context, client *http.Client, page int) (string, error) {
	listingURL := fmt.Sprintf("https://www.profession.hu/allasok/%s/%d,10", professionHuCategorySlug, page)

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
