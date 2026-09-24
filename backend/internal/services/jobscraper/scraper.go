// backend/internal/services/jobscraper/scraper.go
package jobscraper

import (
	"context"
	"time"
)

// ScrapedJob is the common shape both the automated profession.hu scraper
// and the manual URL-add flow produce, so RunScrape and the manual-add
// handler can save either one the same way.
type ScrapedJob struct {
	ExternalURL string
	Title       string
	Company     string
	Location    string
	Description string
	PostedAt    *time.Time
}

// Scraper is implemented once per auto-scraped site. Only profession.hu
// implements it today (see the design doc's Scope section for why
// nofluffjobs.com/LinkedIn are manual-add only).
type Scraper interface {
	Site() string
	Scrape(ctx context.Context) ([]ScrapedJob, error)
}

// userAgent identifies this scraper to site operators, per the design doc's
// robots.txt-compliance requirement - an explicit, contactable identity
// rather than pretending to be a browser.
const userAgent = "DevBridgeManager-JobSearch/1.0 (personal use, contact: zoltan@oktatron.com)"
