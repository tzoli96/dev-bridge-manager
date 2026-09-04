package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type ActivityLogHandler struct{}

func NewActivityLogHandler() *ActivityLogHandler {
	return &ActivityLogHandler{}
}

// logActivity is a fire-and-forget write shared by task/comment/attachment handlers.
// Empty strings for fieldName/oldValue/newValue are stored as NULL.
func logActivity(taskID, userID uint, eventType, fieldName, oldValue, newValue string) {
	entry := models.TaskActivityLog{TaskID: taskID, UserID: userID, EventType: eventType}
	if fieldName != "" {
		entry.FieldName = &fieldName
	}
	if oldValue != "" {
		entry.OldValue = &oldValue
	}
	if newValue != "" {
		entry.NewValue = &newValue
	}
	if err := database.GetDB().Create(&entry).Error; err != nil {
		log.Printf("⚠️ Failed to log activity for task %d: %v", taskID, err)
	}
}

// GetTaskHistory - GET /api/v1/projects/:id/tasks/:taskId/history
func (h *ActivityLogHandler) GetTaskHistory(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	var entries []models.TaskActivityLog
	if err := database.GetDB().
		Where("task_id = ?", taskID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading history"})
	}

	userIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	users := loadUsersByIDs(userIDs)

	dtos := make([]models.ActivityLogDTO, 0, len(entries))
	for _, e := range entries {
		dto := models.ActivityLogDTO{
			ID:        models.IDToStr(e.ID),
			TaskID:    models.IDToStr(e.TaskID),
			UserID:    models.IDToStr(e.UserID),
			User:      userRefDTO(e.UserID, users),
			EventType: e.EventType,
			CreatedAt: e.CreatedAt,
		}
		if e.FieldName != nil {
			dto.FieldName = *e.FieldName
		}
		if e.OldValue != nil {
			dto.OldValue = *e.OldValue
		}
		if e.NewValue != nil {
			dto.NewValue = *e.NewValue
		}
		dtos = append(dtos, dto)
	}

	return c.JSON(dtos)
}
