// backend/internal/handlers/job_search_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

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
