// backend/internal/services/job_scraping.go
package services

import (
	"context"
	"log"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services/jobscraper"

	"gorm.io/gorm"
)

// RunScrape runs every registered scraper, saves any new listings (dedup
// via the (site, external_url) unique constraint), then scores every
// listing that doesn't have a match yet. Mirrors RunGmailSync's per-item
// error tolerance: one scraper (or one match) failing is logged and
// skipped, the rest of the run continues.
func RunScrape(ctx context.Context, matcher JobMatcher) (newListings int, newMatches int) {
	db := database.GetDB()
	scrapers := registeredScrapers(db)

	for _, scraper := range scrapers {
		jobs, err := scraper.Scrape(ctx)
		if err != nil {
			log.Printf("job scraping: %s failed: %v", scraper.Site(), err)
			continue
		}
		for _, job := range jobs {
			result := db.Exec(`
				INSERT INTO job_listings (site, external_url, title, company, location, description, posted_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (site, external_url) DO NOTHING
			`, scraper.Site(), job.ExternalURL, job.Title, job.Company, job.Location, job.Description, job.PostedAt)
			if result.Error != nil {
				log.Printf("job scraping: failed to save listing %s: %v", job.ExternalURL, result.Error)
				continue
			}
			if result.RowsAffected > 0 {
				newListings++
			}
		}
	}

	newMatches = scoreUnmatchedListings(ctx, db, matcher)
	return newListings, newMatches
}

// registeredScrapers builds the one scraper this feature currently
// supports, pre-loaded with the (site, external_url) pairs already in
// job_listings so ProfessionHuScraper.Scrape can stop paging once it stops
// seeing new ads.
func registeredScrapers(db *gorm.DB) []jobscraper.Scraper {
	var professionHuURLs []string
	db.Model(&models.JobListing{}).Where("site = ?", "profession.hu").Pluck("external_url", &professionHuURLs)
	known := make(map[string]bool, len(professionHuURLs))
	for _, u := range professionHuURLs {
		known[u] = true
	}
	return []jobscraper.Scraper{jobscraper.NewProfessionHuScraper(known)}
}

func scoreUnmatchedListings(ctx context.Context, db *gorm.DB, matcher JobMatcher) int {
	var profile models.JobSearchProfile
	if err := db.First(&profile, 1).Error; err != nil {
		log.Printf("job scraping: failed to load job search profile: %v", err)
		return 0
	}

	var listings []models.JobListing
	if err := db.Joins("LEFT JOIN job_matches ON job_matches.job_listing_id = job_listings.id").
		Where("job_matches.id IS NULL").Find(&listings).Error; err != nil {
		log.Printf("job scraping: failed to load unmatched listings: %v", err)
		return 0
	}

	created := 0
	for _, listing := range listings {
		score, reasoning, err := matcher.MatchJob(ctx, profile.CVText, profile.Skills, profile.Preferences, listing.Title, listing.Company, listing.Location, listing.Description)
		if err != nil {
			log.Printf("job scraping: failed to score listing %d: %v", listing.ID, err)
			continue
		}
		if err := db.Create(&models.JobMatch{
			JobListingID: listing.ID,
			Score:        score,
			Reasoning:    reasoning,
			Status:       "new",
		}).Error; err != nil {
			log.Printf("job scraping: failed to save match for listing %d: %v", listing.ID, err)
			continue
		}
		created++
	}
	return created
}
