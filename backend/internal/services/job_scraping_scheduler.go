// backend/internal/services/job_scraping_scheduler.go
package services

import (
	"context"
	"time"
)

const jobScrapingInterval = 24 * time.Hour

// scheduledScrapeMaxPages is much higher than ScanNow's cap (see
// job_search_handler.go's scanNowMaxPages) because this runs in the
// background with no request timeout to protect - it can take as long as
// it needs to reach every listing the site has, relying on the scraper's
// own "page fully known" / "empty page" stop conditions to end the run.
const scheduledScrapeMaxPages = 50

// scheduledNfjMaxJobs caps how many individual NoFluffJobs job pages get
// fetched per run (one HTTP request each, after the category-page crawl) -
// see NoFluffJobsScraper's doc comment for why this can't paginate further
// anyway. Only wired into the scheduled run, not ScanNow (see ScanNow's
// comment in job_search_handler.go).
const scheduledNfjMaxJobs = 80

// StartJobScrapingScheduler mirrors StartGmailSyncScheduler's plain
// time.Ticker pattern: no cron dependency exists in this codebase, and a
// daily cadence matches the design doc's stated scan frequency.
func StartJobScrapingScheduler() {
	matcher := NewJobMatchService()
	RunScrape(context.Background(), matcher, scheduledScrapeMaxPages, scheduledNfjMaxJobs)

	ticker := time.NewTicker(jobScrapingInterval)
	for range ticker.C {
		RunScrape(context.Background(), matcher, scheduledScrapeMaxPages, scheduledNfjMaxJobs)
	}
}
