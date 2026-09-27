// backend/internal/handlers/project_renewal_handler.go
package handlers

import (
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

type ProjectRenewalHandler struct{}

func NewProjectRenewalHandler() *ProjectRenewalHandler {
	return &ProjectRenewalHandler{}
}

// ListProjectRenewalFlags - GET /api/v1/admin/project-renewal-flags?status=pending
// - minden közelgő szerződés-lejárat jelzés, opcionális állapot-szűréssel, a
// dashboard renewal widgetjéhez. super_admin only (see
// routes/project_renewal_routes.go).
func (h *ProjectRenewalHandler) ListProjectRenewalFlags(c *fiber.Ctx) error {
	var rows []struct {
		models.ProjectRenewalFlag
		ProjectName string `gorm:"column:project_name"`
	}

	query := database.GetDB().Table("project_renewal_flags").
		Select("project_renewal_flags.*, projects.name as project_name").
		Joins("LEFT JOIN projects ON project_renewal_flags.project_id = projects.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("project_renewal_flags.status = ?", status)
	}

	if err := query.Order("project_renewal_flags.contract_end_date ASC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(models.ProjectRenewalFlagListResponse{Success: false, Message: "Error fetching project renewal flags"})
	}

	flags := make([]models.ProjectRenewalFlagWithNames, 0, len(rows))
	for _, row := range rows {
		flags = append(flags, models.ProjectRenewalFlagWithNames{
			ProjectRenewalFlag: row.ProjectRenewalFlag,
			ProjectName:        row.ProjectName,
		})
	}

	return c.JSON(models.ProjectRenewalFlagListResponse{Success: true, Flags: flags})
}

// Dismiss - POST /api/v1/admin/project-renewal-flags/:id/dismiss - elnyomja
// ezt a jelzést; a scheduler projectRenewalRepeatInterval elteltével újra
// megnézi, ha a szerződés még mindig nincs megújítva.
func (h *ProjectRenewalHandler) Dismiss(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	flagID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectRenewalFlagActionResponse{Success: false, Message: "Invalid flag ID"})
	}

	db := database.GetDB()

	var flag models.ProjectRenewalFlag
	if err := db.Where("id = ? AND status = ?", flagID, "pending").First(&flag).Error; err != nil {
		return c.Status(404).JSON(models.ProjectRenewalFlagActionResponse{Success: false, Message: "Pending flag not found"})
	}

	dismissedAt := time.Now()
	if err := db.Model(&flag).Updates(map[string]interface{}{
		"status":       "dismissed",
		"dismissed_by": currentUserID,
		"dismissed_at": dismissedAt,
	}).Error; err != nil {
		return c.Status(500).JSON(models.ProjectRenewalFlagActionResponse{Success: false, Message: "Failed to dismiss flag"})
	}
	flag.Status = "dismissed"
	flag.DismissedBy = &currentUserID
	flag.DismissedAt = &dismissedAt

	return c.JSON(models.ProjectRenewalFlagActionResponse{Success: true, Flag: &flag})
}
