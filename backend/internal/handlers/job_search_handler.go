// backend/internal/handlers/job_search_handler.go
package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"dev-bridge-manager/internal/services/jobscraper"

	"github.com/gofiber/fiber/v2"
)

type JobSearchHandler struct {
	matcher services.JobMatcher
	drafter services.JobApplicationDrafter
}

func NewJobSearchHandler() *JobSearchHandler {
	return &JobSearchHandler{
		matcher: services.NewJobMatchService(),
		drafter: services.NewJobApplicationDraftService(),
	}
}

// GetJobSearchProfile - GET /api/v1/admin/job-search/profile (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) GetJobSearchProfile(c *fiber.Ctx) error {
	var profile models.JobSearchProfile
	if err := database.GetDB().First(&profile, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load job search profile"})
	}
	return c.JSON(fiber.Map{"success": true, "profile": profile})
}

// UpdateJobSearchProfile - PUT /api/v1/admin/job-search/profile (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) UpdateJobSearchProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var req struct {
		CVText      string `json:"cv_text"`
		Skills      string `json:"skills"`
		Preferences string `json:"preferences"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	db := database.GetDB()
	if err := db.Model(&models.JobSearchProfile{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"cv_text":     req.CVText,
		"skills":      req.Skills,
		"preferences": req.Preferences,
		"updated_by":  userID,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update job search profile"})
	}

	var profile models.JobSearchProfile
	if err := db.First(&profile, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load updated job search profile"})
	}
	return c.JSON(fiber.Map{"success": true, "profile": profile})
}

// ScanNow - POST /api/v1/admin/job-search/scan-now (super_admin only, see routes/job_search_routes.go)
// Runs RunScrape synchronously and reports how many new listings/matches
// were created - no queue, matching the design doc's stated low volume.
func (h *JobSearchHandler) ScanNow(c *fiber.Ctx) error {
	newListings, newMatches := services.RunScrape(c.Context(), h.matcher)
	return c.JSON(fiber.Map{"success": true, "new_listings": newListings, "new_matches": newMatches})
}

// AddManualListing - POST /api/v1/admin/job-search/listings/manual (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) AddManualListing(c *fiber.Ctx) error {
	var req struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Company     string `json:"company"`
		Location    string `json:"location"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "A hirdetés URL-je kötelező"})
	}

	fetchResult, fetchErr := jobscraper.FetchJobFromURL(c.Context(), req.URL)

	job := jobscraper.ScrapedJob{ExternalURL: req.URL}
	if fetchErr == nil && fetchResult.Extracted {
		job = fetchResult.Job
	} else {
		if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Description) == "" {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"message": "Az adatok automatikus kinyerése nem sikerült - add meg kézzel a hirdetés adatait",
			})
		}
		job.Title = req.Title
		job.Company = req.Company
		job.Location = req.Location
		job.Description = req.Description
	}

	site, err := siteFromURL(req.URL)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Érvénytelen URL"})
	}

	db := database.GetDB()
	var listing models.JobListing
	err = db.Raw(`
		INSERT INTO job_listings (site, external_url, title, company, location, description)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (site, external_url) DO NOTHING
		RETURNING id, site, external_url, title, company, location, description, posted_at, scraped_at
	`, site, job.ExternalURL, job.Title, job.Company, job.Location, job.Description).Scan(&listing).Error
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to save listing"})
	}
	if listing.ID == 0 {
		// ON CONFLICT DO NOTHING left listing.ID unset - the row already
		// existed, so load it back by its unique key.
		if err := db.Where("site = ? AND external_url = ?", site, job.ExternalURL).First(&listing).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load existing listing"})
		}
	}

	var match models.JobMatch
	if err := db.Where("job_listing_id = ?", listing.ID).First(&match).Error; err != nil {
		var profile models.JobSearchProfile
		if err := db.First(&profile, 1).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load job search profile"})
		}
		score, reasoning, err := h.matcher.MatchJob(c.Context(), profile.CVText, profile.Skills, profile.Preferences, listing.Title, listing.Company, listing.Location, listing.Description)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to score listing: " + err.Error()})
		}
		match = models.JobMatch{JobListingID: listing.ID, Score: score, Reasoning: reasoning, Status: "new"}
		if err := db.Create(&match).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to save match"})
		}
	}
	match.JobListing = listing

	return c.JSON(fiber.Map{"success": true, "match": match})
}

func siteFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}
	return strings.TrimPrefix(u.Host, "www."), nil
}

// ListJobMatches - GET /api/v1/admin/job-search/matches?status= (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) ListJobMatches(c *fiber.Ctx) error {
	query := database.GetDB().Preload("JobListing").Order("score DESC")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	var matches []models.JobMatch
	if err := query.Find(&matches).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load matches"})
	}
	return c.JSON(fiber.Map{"success": true, "matches": matches})
}

var validJobMatchStatuses = map[string]bool{"reviewed": true, "dismissed": true}

// UpdateJobMatchStatus - PATCH /api/v1/admin/job-search/matches/:id/status (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) UpdateJobMatchStatus(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid match id"})
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil || !validJobMatchStatuses[req.Status] {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Status must be 'reviewed' or 'dismissed'"})
	}
	if err := database.GetDB().Model(&models.JobMatch{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update status"})
	}
	return c.JSON(fiber.Map{"success": true})
}

// DraftApplication - POST /api/v1/admin/job-search/matches/:id/draft-application (super_admin only, see routes/job_search_routes.go)
// Generates a draft without persisting it, same "generate only" pattern as
// email_handler.go's BreakdownEmailIntoTasks.
func (h *JobSearchHandler) DraftApplication(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid match id"})
	}
	var req struct {
		Instruction string `json:"instruction"`
	}
	c.BodyParser(&req)

	var match models.JobMatch
	if err := database.GetDB().Preload("JobListing").First(&match, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Match not found"})
	}

	var profile models.JobSearchProfile
	if err := database.GetDB().First(&profile, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load job search profile"})
	}

	draft, err := h.drafter.DraftApplication(c.Context(), profile.CVText, profile.Skills, match.JobListing.Title, match.JobListing.Company, match.JobListing.Description, req.Instruction)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to generate application draft: " + err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "draft": draft})
}

// MarkApplied - POST /api/v1/admin/job-search/matches/:id/mark-applied (super_admin only, see routes/job_search_routes.go)
// Called after either the "sent via email" or "applied manually on site"
// path completes on the frontend.
func (h *JobSearchHandler) MarkApplied(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid match id"})
	}
	var req struct {
		ApplicationText string `json:"application_text"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	now := time.Now()
	if err := database.GetDB().Model(&models.JobMatch{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":           "applied",
		"applied_at":       now,
		"application_text": req.ApplicationText,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to mark as applied"})
	}
	return c.JSON(fiber.Map{"success": true})
}
