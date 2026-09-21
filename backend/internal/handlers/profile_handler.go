// backend/internal/handlers/profile_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ProfileHandler struct{}

func NewProfileHandler() *ProfileHandler {
	return &ProfileHandler{}
}

func loadProfile(db *gorm.DB) (*models.Profile, error) {
	var profile models.Profile
	if err := db.Preload("Samples").First(&profile, 1).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetProfile - GET /api/v1/admin/profile (super_admin only, see routes/profile_routes.go)
func (h *ProfileHandler) GetProfile(c *fiber.Ctx) error {
	profile, err := loadProfile(database.GetDB())
	if err != nil {
		return c.Status(500).JSON(models.ProfileResponse{Success: false, Message: "Failed to load profile"})
	}
	return c.JSON(models.ProfileResponse{Success: true, Profile: profile})
}

// UpdateProfile - PUT /api/v1/admin/profile (super_admin only, see routes/profile_routes.go)
// Replaces the full samples list on every call - simpler than diffing, and
// the admin UI always submits the complete list back anyway.
func (h *ProfileHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var req models.ProfileUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProfileResponse{Success: false, Message: "Invalid request body"})
	}

	db := database.GetDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Profile{}).Where("id = ?", 1).Updates(map[string]interface{}{
			"background": req.Background,
			"expertise":  req.Expertise,
			"tone_rules": req.ToneRules,
			"updated_by": userID,
		}).Error; err != nil {
			return err
		}

		if err := tx.Where("profile_id = ?", 1).Delete(&models.ProfileSample{}).Error; err != nil {
			return err
		}

		for _, s := range req.Samples {
			if err := tx.Create(&models.ProfileSample{ProfileID: 1, Label: s.Label, Content: s.Content}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return c.Status(500).JSON(models.ProfileResponse{Success: false, Message: "Failed to update profile"})
	}

	profile, err := loadProfile(db)
	if err != nil {
		return c.Status(500).JSON(models.ProfileResponse{Success: false, Message: "Failed to load updated profile"})
	}
	return c.JSON(models.ProfileResponse{Success: true, Profile: profile})
}

// GetProfileContext - GET /api/v1/internal/profile-context - called by other
// services on the docker-internal network, not by end users; unauthenticated,
// mirroring the AI service's own unauthenticated endpoints (network
// isolation is the guard, same accepted risk model as ai:8000 today).
func (h *ProfileHandler) GetProfileContext(c *fiber.Ctx) error {
	profile, err := loadProfile(database.GetDB())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"context": ""})
	}
	return c.JSON(fiber.Map{"context": services.BuildProfileContext(profile)})
}
