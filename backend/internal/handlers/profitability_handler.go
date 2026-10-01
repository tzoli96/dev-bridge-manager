// backend/internal/handlers/profitability_handler.go
package handlers

import (
	"math"
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
)

type ProfitabilityHandler struct{}

func NewProfitabilityHandler() *ProfitabilityHandler {
	return &ProfitabilityHandler{}
}

// parseOverviewMonths returns the requested window length; empty means the
// default of 3 months, anything outside 1..24 is rejected.
func parseOverviewMonths(raw string) (int, bool) {
	if raw == "" {
		return 3, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 24 {
		return 0, false
	}
	return n, true
}

// finiteBetween reports lo <= v <= hi and is false for NaN and +-Inf.
func finiteBetween(v, lo, hi float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= lo && v <= hi
}

func validateProfitSettings(req models.ProfitSettingsRequest) string {
	if !finiteBetween(req.MinutesPerInboundEmail, 0, 240) ||
		!finiteBetween(req.MinutesPerOutboundEmail, 0, 240) {
		return "Email minutes must be between 0 and 240"
	}
	if !finiteBetween(req.DefaultCapacityHoursPerMonth, 0, 744) {
		return "Capacity must be between 0 and 744 hours per month"
	}
	if !finiteBetween(req.UnderpricedRatioThreshold, 0.01, 1.5) {
		return "Threshold must be between 0.01 and 1.5"
	}
	return ""
}

func validateMeetingAllowance(req models.MeetingAllowanceRequest) string {
	if !finiteBetween(req.HoursPerMonth, 0, 744) {
		return "Hours per month must be between 0 and 744"
	}
	return ""
}

// GetOverview - GET /api/v1/profitability/overview?months=
func (h *ProfitabilityHandler) GetOverview(c *fiber.Ctx) error {
	months, ok := parseOverviewMonths(c.Query("months"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "months must be between 1 and 24"})
	}
	overview, err := services.LoadOverview(months)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading profitability overview"})
	}
	return c.JSON(overview)
}

// GetSettings - GET /api/v1/profitability/settings
func (h *ProfitabilityHandler) GetSettings(c *fiber.Ctx) error {
	var settings models.ProfitSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading settings"})
	}
	return c.JSON(settings)
}

// UpdateSettings - PUT /api/v1/profitability/settings
func (h *ProfitabilityHandler) UpdateSettings(c *fiber.Ctx) error {
	var req models.ProfitSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateProfitSettings(req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	var settings models.ProfitSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading settings"})
	}
	settings.MinutesPerInboundEmail = req.MinutesPerInboundEmail
	settings.MinutesPerOutboundEmail = req.MinutesPerOutboundEmail
	settings.DefaultCapacityHoursPerMonth = req.DefaultCapacityHoursPerMonth
	settings.UnderpricedRatioThreshold = req.UnderpricedRatioThreshold
	settings.UpdatedAt = time.Now()
	if err := database.GetDB().Save(&settings).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating settings"})
	}
	return c.JSON(settings)
}

// PutMeetingAllowance - PUT /api/v1/profitability/clients/:id/meeting-allowance
func (h *ProfitabilityHandler) PutMeetingAllowance(c *fiber.Ctx) error {
	clientID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid client ID"})
	}
	var req models.MeetingAllowanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateMeetingAllowance(req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	var client models.Client
	if err := database.GetDB().Select("id").First(&client, clientID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Client not found"})
	}

	allowance := models.ClientMeetingAllowance{
		ClientID:      uint(clientID),
		HoursPerMonth: req.HoursPerMonth,
		UpdatedAt:     time.Now(),
	}
	err = database.GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"hours_per_month", "updated_at"}),
	}).Create(&allowance).Error
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error saving meeting allowance"})
	}
	return c.JSON(allowance)
}

// parseForecastMonths returns the requested horizon; empty means the default
// of 6 months, anything outside 3..6 is rejected.
func parseForecastMonths(raw string) (int, bool) {
	if raw == "" {
		return 6, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 3 || n > 6 {
		return 0, false
	}
	return n, true
}

// GetForecast - GET /api/v1/profitability/forecast?months=
func (h *ProfitabilityHandler) GetForecast(c *fiber.Ctx) error {
	months, ok := parseForecastMonths(c.Query("months"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "months must be between 3 and 6"})
	}
	forecast, err := services.LoadForecast(months)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading profitability forecast"})
	}
	return c.JSON(forecast)
}
