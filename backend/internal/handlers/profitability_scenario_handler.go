// backend/internal/handlers/profitability_scenario_handler.go
package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

const maxScenarioMoney = 1_000_000_000.0

// validateParameterSetRequest trims the request in place and returns a
// user-facing message for the first problem found, or "" when it is valid.
func validateParameterSetRequest(req *models.ProfitParameterSetRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "Name is required"
	}
	if utf8.RuneCountInString(req.Name) > 255 {
		return "Name is too long"
	}
	if len(req.PercentItems) > 20 || len(req.FixedMonthlyCosts) > 20 {
		return "At most 20 items per list"
	}
	for i := range req.PercentItems {
		it := &req.PercentItems[i]
		it.Label = strings.TrimSpace(it.Label)
		if it.Label == "" || utf8.RuneCountInString(it.Label) > 100 {
			return "Each percent item needs a label (max 100 characters)"
		}
		if !finiteBetween(it.Percent, 0, 100) {
			return "Percent must be between 0 and 100"
		}
		if it.Base != services.PercentBaseRevenue && it.Base != services.PercentBaseAfterCosts {
			return "Base must be revenue or after_costs"
		}
	}
	for i := range req.FixedMonthlyCosts {
		c := &req.FixedMonthlyCosts[i]
		c.Label = strings.TrimSpace(c.Label)
		if c.Label == "" || utf8.RuneCountInString(c.Label) > 100 {
			return "Each fixed cost needs a label (max 100 characters)"
		}
		if !finiteBetween(c.Amount, 0, maxScenarioMoney) {
			return "Fixed cost must be between 0 and 1000000000"
		}
	}
	return ""
}

// GetParameterSets - GET /api/v1/profitability/parameter-sets
func (h *ProfitabilityHandler) GetParameterSets(c *fiber.Ctx) error {
	var sets []models.ProfitParameterSet
	if err := database.GetDB().Order("name ASC, id ASC").Find(&sets).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading parameter sets"})
	}
	return c.JSON(sets)
}

// CreateParameterSet - POST /api/v1/profitability/parameter-sets
func (h *ProfitabilityHandler) CreateParameterSet(c *fiber.Ctx) error {
	var req models.ProfitParameterSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateParameterSetRequest(&req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	set := models.ProfitParameterSet{
		Name:              req.Name,
		PercentItems:      models.JSONList[models.ProfitPercentItem](req.PercentItems),
		FixedMonthlyCosts: models.JSONList[models.ProfitFixedCost](req.FixedMonthlyCosts),
		CreatedBy:         currentUserID(c),
	}
	if err := database.GetDB().Create(&set).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating parameter set"})
	}
	return c.Status(201).JSON(set)
}

// UpdateParameterSet - PUT /api/v1/profitability/parameter-sets/:id
func (h *ProfitabilityHandler) UpdateParameterSet(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid parameter set ID"})
	}
	var set models.ProfitParameterSet
	if err := database.GetDB().First(&set, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	var req models.ProfitParameterSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateParameterSetRequest(&req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	set.Name = req.Name
	set.PercentItems = models.JSONList[models.ProfitPercentItem](req.PercentItems)
	set.FixedMonthlyCosts = models.JSONList[models.ProfitFixedCost](req.FixedMonthlyCosts)
	if err := database.GetDB().Save(&set).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating parameter set"})
	}
	return c.JSON(set)
}

// DeleteParameterSet - DELETE /api/v1/profitability/parameter-sets/:id
// Refuses with 409 while scenarios still reference the set (the database
// foreign key is ON DELETE RESTRICT as well).
func (h *ProfitabilityHandler) DeleteParameterSet(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid parameter set ID"})
	}

	var used int64
	if err := database.GetDB().Model(&models.ProfitScenario{}).Where("parameter_set_id = ?", id).Count(&used).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting parameter set"})
	}
	if used > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Parameter set is used by %d scenario(s)", used)})
	}

	result := database.GetDB().Delete(&models.ProfitParameterSet{}, id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting parameter set"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Parameter set deleted successfully"})
}

// validateScenarioRequest trims the request in place and returns a
// user-facing message for the first problem found, or "" when it is valid.
// The name is only required when the scenario is saved, not for a live compute.
func validateScenarioRequest(req *models.ProfitScenarioRequest, requireName bool) string {
	req.Name = strings.TrimSpace(req.Name)
	if requireName && req.Name == "" {
		return "Name is required"
	}
	if utf8.RuneCountInString(req.Name) > 255 {
		return "Name is too long"
	}
	if req.HorizonMonths < 3 || req.HorizonMonths > 6 {
		return "Horizon must be between 3 and 6 months"
	}
	if req.ParameterSetID == 0 {
		return "Parameter set is required"
	}
	if !(finiteBetween(req.CapacityHoursPerMonth, 0, 744) && req.CapacityHoursPerMonth > 0) {
		return "Capacity must be greater than 0 and at most 744 hours per month"
	}
	if len(req.ClientAdjustments) > 200 {
		return "At most 200 client adjustments"
	}
	if len(req.NewClients) > 50 {
		return "At most 50 new clients"
	}

	seen := make(map[uint]bool, len(req.ClientAdjustments))
	for _, a := range req.ClientAdjustments {
		if a.ClientID == 0 {
			return "Each adjustment needs a client"
		}
		if seen[a.ClientID] {
			return "A client can only be adjusted once"
		}
		seen[a.ClientID] = true
		if a.NewHourlyRate != nil && a.NewFixedPrice != nil {
			return "Set either a new hourly rate or a new fixed price, not both"
		}
		if a.NewHourlyRate != nil && !finiteBetween(*a.NewHourlyRate, 0, maxScenarioMoney) {
			return "Hourly rate must be between 0 and 1000000000"
		}
		if a.NewFixedPrice != nil && !finiteBetween(*a.NewFixedPrice, 0, maxScenarioMoney) {
			return "Fixed price must be between 0 and 1000000000"
		}
		if !finiteBetween(a.HoursDelta, -744, 744) {
			return "Hours change must be between -744 and 744"
		}
	}

	for i := range req.NewClients {
		n := &req.NewClients[i]
		n.Name = strings.TrimSpace(n.Name)
		if n.Name == "" || utf8.RuneCountInString(n.Name) > 255 {
			return "Each new client needs a name (max 255 characters)"
		}
		if !finiteBetween(n.MonthlyRevenue, 0, maxScenarioMoney) {
			return "New client revenue must be between 0 and 1000000000"
		}
		if !finiteBetween(n.MonthlyHours, 0, 744) {
			return "New client hours must be between 0 and 744"
		}
	}
	return ""
}

// parameterSetExists reports whether the referenced parameter set is there.
func parameterSetExists(id uint) bool {
	var count int64
	database.GetDB().Model(&models.ProfitParameterSet{}).Where("id = ?", id).Count(&count)
	return count > 0
}

// GetScenarios - GET /api/v1/profitability/scenarios
func (h *ProfitabilityHandler) GetScenarios(c *fiber.Ctx) error {
	var scenarios []models.ProfitScenario
	if err := database.GetDB().Order("created_at DESC, id DESC").Find(&scenarios).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading scenarios"})
	}
	return c.JSON(scenarios)
}

// CreateScenario - POST /api/v1/profitability/scenarios
func (h *ProfitabilityHandler) CreateScenario(c *fiber.Ctx) error {
	var req models.ProfitScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateScenarioRequest(&req, true); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}
	if !parameterSetExists(req.ParameterSetID) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	scenario := models.ProfitScenario{
		Name:                  req.Name,
		HorizonMonths:         req.HorizonMonths,
		ParameterSetID:        req.ParameterSetID,
		CapacityHoursPerMonth: req.CapacityHoursPerMonth,
		ClientAdjustments:     models.JSONList[models.ProfitScenarioAdjustment](req.ClientAdjustments),
		NewClients:            models.JSONList[models.ProfitScenarioNewClient](req.NewClients),
		CreatedBy:             currentUserID(c),
	}
	if err := database.GetDB().Create(&scenario).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating scenario"})
	}
	return c.Status(201).JSON(scenario)
}

// UpdateScenario - PUT /api/v1/profitability/scenarios/:id
func (h *ProfitabilityHandler) UpdateScenario(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid scenario ID"})
	}
	var scenario models.ProfitScenario
	if err := database.GetDB().First(&scenario, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Scenario not found"})
	}

	var req models.ProfitScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateScenarioRequest(&req, true); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}
	if !parameterSetExists(req.ParameterSetID) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	scenario.Name = req.Name
	scenario.HorizonMonths = req.HorizonMonths
	scenario.ParameterSetID = req.ParameterSetID
	scenario.CapacityHoursPerMonth = req.CapacityHoursPerMonth
	scenario.ClientAdjustments = models.JSONList[models.ProfitScenarioAdjustment](req.ClientAdjustments)
	scenario.NewClients = models.JSONList[models.ProfitScenarioNewClient](req.NewClients)
	if err := database.GetDB().Save(&scenario).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating scenario"})
	}
	return c.JSON(scenario)
}

// DeleteScenario - DELETE /api/v1/profitability/scenarios/:id
func (h *ProfitabilityHandler) DeleteScenario(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid scenario ID"})
	}
	result := database.GetDB().Delete(&models.ProfitScenario{}, id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting scenario"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Scenario not found"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Scenario deleted successfully"})
}

// ComputeScenario - POST /api/v1/profitability/scenarios/compute
// Prices an unsaved scenario draft (nothing is written).
func (h *ProfitabilityHandler) ComputeScenario(c *fiber.Ctx) error {
	var req models.ProfitScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateScenarioRequest(&req, false); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	var set models.ProfitParameterSet
	if err := database.GetDB().First(&set, req.ParameterSetID).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	result, err := services.ComputeScenario(services.ScenarioInput{
		CapacityHours: req.CapacityHoursPerMonth,
		Parameters:    set,
		Adjustments:   req.ClientAdjustments,
		NewClients:    req.NewClients,
	}, req.HorizonMonths)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error computing scenario"})
	}
	return c.JSON(result)
}
