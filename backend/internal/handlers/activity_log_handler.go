package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"encoding/json"
	"log"
	"strconv"
	"strings"

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

func tagNamesFromJSON(raw string) string {
	if raw == "" {
		return ""
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return ""
	}
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

// commentPreview truncates a comment's content to at most 80 runes for
// display in the activity log, appending an ellipsis when truncated.
// Rune-safe: slicing on bytes can cut a multi-byte UTF-8 character in half
// and produce invalid UTF-8 that Postgres rejects on insert.
func commentPreview(s string) string {
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return s
}

func assigneeName(id *uint, users map[uint]models.User) string {
	if id == nil {
		return ""
	}
	if u, ok := users[*id]; ok {
		return u.Name
	}
	return ""
}

// logTaskFieldChanges diffs the task before/after UpdateTask's mutation and logs one
// field_changed row per field that actually changed.
func logTaskFieldChanges(original, updated models.Task, userID uint) {
	logField := func(field, oldVal, newVal string) {
		if oldVal == newVal {
			return
		}
		logActivity(updated.ID, userID, "field_changed", field, oldVal, newVal)
	}

	logField("title", original.Title, updated.Title)
	logField("description", original.Description, updated.Description)
	logField("priority", original.Priority, updated.Priority)
	logField("dueDate", models.FormatDate(original.DueDate), models.FormatDate(updated.DueDate))
	logField("estimatedHours",
		strconv.FormatFloat(original.EstimatedHours, 'f', -1, 64),
		strconv.FormatFloat(updated.EstimatedHours, 'f', -1, 64))
	logField("tags", tagNamesFromJSON(original.Tags), tagNamesFromJSON(updated.Tags))

	oldAssignee, newAssignee := original.AssigneeID, updated.AssigneeID
	changed := (oldAssignee == nil) != (newAssignee == nil)
	if !changed && oldAssignee != nil && newAssignee != nil {
		changed = *oldAssignee != *newAssignee
	}
	if changed {
		ids := make([]uint, 0, 2)
		if oldAssignee != nil {
			ids = append(ids, *oldAssignee)
		}
		if newAssignee != nil {
			ids = append(ids, *newAssignee)
		}
		users := loadUsersByIDs(ids)
		logField("assignee", assigneeName(oldAssignee, users), assigneeName(newAssignee, users))
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
