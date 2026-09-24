// backend/internal/services/job_scraping_scheduler.go
package services

import (
	"context"
	"time"
)

const jobScrapingInterval = 24 * time.Hour

// StartJobScrapingScheduler mirrors StartGmailSyncScheduler's plain
// time.Ticker pattern: no cron dependency exists in this codebase, and a
// daily cadence matches the design doc's stated scan frequency.
func StartJobScrapingScheduler() {
	matcher := NewJobMatchService()
	RunScrape(context.Background(), matcher)

	ticker := time.NewTicker(jobScrapingInterval)
	for range ticker.C {
		RunScrape(context.Background(), matcher)
	}
}
