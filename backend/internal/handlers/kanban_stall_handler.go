// backend/internal/handlers/kanban_stall_handler.go
package handlers

import (
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

type KanbanStallHandler struct{}

func NewKanbanStallHandler() *KanbanStallHandler {
	return &KanbanStallHandler{}
}

// ListKanbanStallFlags - GET /api/v1/admin/kanban-stall-flags?status=pending
// - minden megakadt feladat jelzés, opcionális állapot-szűréssel, a
// dashboard "Megakadt feladatok" widgetjéhez. super_admin only (see
// routes/kanban_stall_routes.go).
func (h *KanbanStallHandler) ListKanbanStallFlags(c *fiber.Ctx) error {
	var rows []struct {
		models.KanbanStallFlag
		TaskTitle   string `gorm:"column:task_title"`
		ProjectName string `gorm:"column:project_name"`
		BoardName   string `gorm:"column:board_name"`
		ColumnTitle string `gorm:"column:column_title"`
	}

	query := database.GetDB().Table("kanban_stall_flags").
		Select("kanban_stall_flags.*, tasks.title as task_title, projects.name as project_name, boards.name as board_name, kanban_columns.title as column_title").
		Joins("LEFT JOIN tasks ON kanban_stall_flags.task_id = tasks.id").
		Joins("LEFT JOIN projects ON kanban_stall_flags.project_id = projects.id").
		Joins("LEFT JOIN boards ON kanban_stall_flags.board_id = boards.id").
		Joins("LEFT JOIN kanban_columns ON kanban_stall_flags.column_id = kanban_columns.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("kanban_stall_flags.status = ?", status)
	}

	if err := query.Order("kanban_stall_flags.created_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(models.KanbanStallFlagListResponse{Success: false, Message: "Error fetching stalled task flags"})
	}

	flags := make([]models.KanbanStallFlagWithNames, 0, len(rows))
	for _, row := range rows {
		flags = append(flags, models.KanbanStallFlagWithNames{
			KanbanStallFlag: row.KanbanStallFlag,
			TaskTitle:       row.TaskTitle,
			ProjectName:     row.ProjectName,
			BoardName:       row.BoardName,
			ColumnTitle:     row.ColumnTitle,
		})
	}

	return c.JSON(models.KanbanStallFlagListResponse{Success: true, Flags: flags})
}

// Dismiss - POST /api/v1/admin/kanban-stall-flags/:id/dismiss - elnyomja ezt
// a jelzést; a scheduler kanbanStallRepeatInterval elteltével újra
// megnézi, ha a feladat még mindig ugyanabban az oszlopban van.
func (h *KanbanStallHandler) Dismiss(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	flagID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.KanbanStallFlagActionResponse{Success: false, Message: "Invalid flag ID"})
	}

	db := database.GetDB()

	var flag models.KanbanStallFlag
	if err := db.Where("id = ? AND status = ?", flagID, "pending").First(&flag).Error; err != nil {
		return c.Status(404).JSON(models.KanbanStallFlagActionResponse{Success: false, Message: "Pending flag not found"})
	}

	dismissedAt := time.Now()
	if err := db.Model(&flag).Updates(map[string]interface{}{
		"status":       "dismissed",
		"dismissed_by": currentUserID,
		"dismissed_at": dismissedAt,
	}).Error; err != nil {
		return c.Status(500).JSON(models.KanbanStallFlagActionResponse{Success: false, Message: "Failed to dismiss flag"})
	}
	flag.Status = "dismissed"
	flag.DismissedBy = &currentUserID
	flag.DismissedAt = &dismissedAt

	return c.JSON(models.KanbanStallFlagActionResponse{Success: true, Flag: &flag})
}
