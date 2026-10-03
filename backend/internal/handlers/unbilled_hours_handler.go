// backend/internal/handlers/unbilled_hours_handler.go
package handlers

import (
	"log"
	"time"

	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type UnbilledHoursHandler struct {
	permissionService *services.PermissionService
}

func NewUnbilledHoursHandler() *UnbilledHoursHandler {
	return &UnbilledHoursHandler{permissionService: services.NewPermissionService()}
}

// GetUnbilledHours - GET /api/v1/billing/unbilled-hours
// Same access rule as the other invoice endpoints (invoices.read, with the
// admin/super_admin/manager fallback). Read-only.
func (h *UnbilledHoursHandler) GetUnbilledHours(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	result, err := services.LoadUnbilledHours(time.Now())
	if err != nil {
		log.Printf("⚠️ unbilled hours: %v", err)
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading unbilled hours"})
	}
	return c.JSON(result)
}
