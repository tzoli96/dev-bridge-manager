// backend/internal/handlers/invoice_reconciliation_handler.go
package handlers

import (
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

type InvoiceReconciliationHandler struct{}

func NewInvoiceReconciliationHandler() *InvoiceReconciliationHandler {
	return &InvoiceReconciliationHandler{}
}

// ListInvoiceReconciliationFlags - GET /api/v1/admin/invoice-reconciliation-flags?status=pending
// - time-tracking vs. invoicing mismatches (billing drift + stale
// uninvoiced hours), for the dashboard widget. super_admin only (see
// routes/invoice_reconciliation_routes.go).
func (h *InvoiceReconciliationHandler) ListInvoiceReconciliationFlags(c *fiber.Ctx) error {
	var rows []struct {
		models.InvoiceReconciliationFlag
		ProjectName string `gorm:"column:project_name"`
	}

	query := database.GetDB().Table("invoice_reconciliation_flags").
		Select("invoice_reconciliation_flags.*, projects.name as project_name").
		Joins("LEFT JOIN projects ON invoice_reconciliation_flags.project_id = projects.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("invoice_reconciliation_flags.status = ?", status)
	}

	if err := query.Order("invoice_reconciliation_flags.created_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(models.InvoiceReconciliationFlagListResponse{Success: false, Message: "Error fetching invoice reconciliation flags"})
	}

	flags := make([]models.InvoiceReconciliationFlagWithNames, 0, len(rows))
	for _, row := range rows {
		flags = append(flags, models.InvoiceReconciliationFlagWithNames{
			InvoiceReconciliationFlag: row.InvoiceReconciliationFlag,
			ProjectName:               row.ProjectName,
		})
	}

	return c.JSON(models.InvoiceReconciliationFlagListResponse{Success: true, Flags: flags})
}

// Dismiss - POST /api/v1/admin/invoice-reconciliation-flags/:id/dismiss -
// elnyomja ezt a jelzést; a scheduler invoiceReconciliationRepeatInterval
// elteltével újra megnézi, ha az eltérés még mindig fennáll.
func (h *InvoiceReconciliationHandler) Dismiss(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	flagID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceReconciliationFlagActionResponse{Success: false, Message: "Invalid flag ID"})
	}

	db := database.GetDB()

	var flag models.InvoiceReconciliationFlag
	if err := db.Where("id = ? AND status = ?", flagID, "pending").First(&flag).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceReconciliationFlagActionResponse{Success: false, Message: "Pending flag not found"})
	}

	dismissedAt := time.Now()
	if err := db.Model(&flag).Updates(map[string]interface{}{
		"status":       "dismissed",
		"dismissed_by": currentUserID,
		"dismissed_at": dismissedAt,
	}).Error; err != nil {
		return c.Status(500).JSON(models.InvoiceReconciliationFlagActionResponse{Success: false, Message: "Failed to dismiss flag"})
	}
	flag.Status = "dismissed"
	flag.DismissedBy = &currentUserID
	flag.DismissedAt = &dismissedAt

	return c.JSON(models.InvoiceReconciliationFlagActionResponse{Success: true, Flag: &flag})
}
