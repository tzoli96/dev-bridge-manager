// backend/internal/handlers/billingo_settings_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type BillingoSettingsHandler struct {
	permissionService *services.PermissionService
	billingoService   *services.BillingoService
}

func NewBillingoSettingsHandler() *BillingoSettingsHandler {
	return &BillingoSettingsHandler{
		permissionService: services.NewPermissionService(),
		billingoService:   services.NewBillingoService(),
	}
}

// checkBillingoSettingsAccess - jogosultság ellenőrzése, admin/super_admin fallback-kel
func (h *BillingoSettingsHandler) checkBillingoSettingsAccess(userID uint, permission string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, permission)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// GetBillingoSettings - GET /api/v1/admin/billingo-settings
func (h *BillingoSettingsHandler) GetBillingoSettings(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkBillingoSettingsAccess(currentUserID, "billingo_settings.manage"); err != nil {
		return err
	}

	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "Billingo settings not found",
		})
	}

	return c.JSON(models.BillingoSettingsResponse{
		Success:              true,
		Message:              "Billingo settings retrieved successfully",
		APIKeyMasked:         maskAPIKey(settings.APIKey),
		BlockID:              settings.BlockID,
		DefaultUnit:          settings.DefaultUnit,
		DefaultUnitPriceType: settings.DefaultUnitPriceType,
		UpdatedBy:            settings.UpdatedBy,
		UpdatedAt:            settings.UpdatedAt,
	})
}

// UpdateBillingoSettings - PUT /api/v1/admin/billingo-settings
func (h *BillingoSettingsHandler) UpdateBillingoSettings(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkBillingoSettingsAccess(currentUserID, "billingo_settings.manage"); err != nil {
		return err
	}

	var req models.BillingoSettingsUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if strings.TrimSpace(req.APIKey) == "" {
		return c.Status(400).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "API key is required",
		})
	}

	if req.DefaultUnitPriceType != "" && req.DefaultUnitPriceType != "net" && req.DefaultUnitPriceType != "gross" {
		return c.Status(400).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "default_unit_price_type must be one of: net, gross",
		})
	}

	updates := map[string]interface{}{
		"api_key":    req.APIKey,
		"block_id":   req.BlockID,
		"updated_by": currentUserID,
	}
	if req.DefaultUnit != "" {
		updates["default_unit"] = req.DefaultUnit
	}
	if req.DefaultUnitPriceType != "" {
		updates["default_unit_price_type"] = req.DefaultUnitPriceType
	}

	if err := database.GetDB().Model(&models.BillingoSettings{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "Error updating billingo settings",
		})
	}

	var settings models.BillingoSettings
	database.GetDB().First(&settings, 1)

	return c.JSON(models.BillingoSettingsResponse{
		Success:              true,
		Message:              "Billingo settings updated successfully",
		APIKeyMasked:         maskAPIKey(settings.APIKey),
		BlockID:              settings.BlockID,
		DefaultUnit:          settings.DefaultUnit,
		DefaultUnitPriceType: settings.DefaultUnitPriceType,
		UpdatedBy:            settings.UpdatedBy,
		UpdatedAt:            settings.UpdatedAt,
	})
}

// ListBillingoBlocks - GET /api/v1/admin/billingo-settings/blocks
// Lists the account's invoice blocks from Billingo using the currently
// saved API key, so the frontend can offer a picker instead of a free-text
// numeric block id field.
func (h *BillingoSettingsHandler) ListBillingoBlocks(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkBillingoSettingsAccess(currentUserID, "billingo_settings.manage"); err != nil {
		return err
	}

	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Billingo settings not found",
		})
	}

	if settings.APIKey == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Save an API key before loading invoice blocks",
		})
	}

	blocks, err := h.billingoService.ListDocumentBlocks(settings.APIKey)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    blocks,
	})
}
