// handlers/task_time_entry_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type TaskTimeEntryHandler struct{}

func NewTaskTimeEntryHandler() *TaskTimeEntryHandler {
	return &TaskTimeEntryHandler{}
}

func timeEntryDTOsForEntries(entries []models.TaskTimeEntry, projectID uint) []models.TimeEntryDTO {
	userIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	users := loadUsersByIDs(userIDs)
	invoicedPeriods := invoicedPeriodsForProject(projectID)

	dtos := make([]models.TimeEntryDTO, 0, len(entries))
	for _, e := range entries {
		dtos = append(dtos, buildTimeEntryDTO(e, users, invoicedPeriods))
	}
	return dtos
}

// GetTaskTimeEntries - GET /api/v1/projects/:id/tasks/:taskId/time-entries
func (h *TaskTimeEntryHandler) GetTaskTimeEntries(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var entries []models.TaskTimeEntry
	if err := database.GetDB().Where("task_id = ?", taskID).Order("date DESC").Find(&entries).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading time entries"})
	}

	return c.JSON(timeEntryDTOsForEntries(entries, taskProjectID(uint(taskID))))
}

// GetProjectTimeEntries - GET /api/v1/projects/:id/time-entries
func (h *TaskTimeEntryHandler) GetProjectTimeEntries(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var taskIDs []uint
	database.GetDB().Model(&models.Task{}).Where("project_id = ?", projectID).Pluck("id", &taskIDs)

	var entries []models.TaskTimeEntry
	if len(taskIDs) > 0 {
		if err := database.GetDB().Where("task_id IN ?", taskIDs).Order("date DESC").Find(&entries).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading time entries"})
		}
	}

	return c.JSON(timeEntryDTOsForEntries(entries, uint(projectID)))
}

// CreateTimeEntry - POST /api/v1/projects/:id/tasks/:taskId/time-entries
func (h *TaskTimeEntryHandler) CreateTimeEntry(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var req models.CreateTimeEntryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Hours <= 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Hours must be greater than 0"})
	}
	date, err := models.ParseDate(req.Date)
	if err != nil || date == nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid date"})
	}

	entry := models.TaskTimeEntry{
		TaskID:      uint(taskID),
		UserID:      currentUserID(c),
		Hours:       req.Hours,
		Description: req.Description,
		Date:        *date,
	}
	if err := database.GetDB().Create(&entry).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating time entry"})
	}

	users := loadUsersByIDs([]uint{entry.UserID})
	invoicedPeriods := invoicedPeriodsForProject(taskProjectID(entry.TaskID))
	return c.Status(201).JSON(buildTimeEntryDTO(entry, users, invoicedPeriods))
}

// UpdateTimeEntry - PUT /api/v1/projects/:id/time-entries/:entryId
func (h *TaskTimeEntryHandler) UpdateTimeEntry(c *fiber.Ctx) error {
	entryID, err := strconv.Atoi(c.Params("entryId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid time entry ID"})
	}

	var entry models.TaskTimeEntry
	if err := database.GetDB().First(&entry, entryID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Time entry not found"})
	}

	var req models.UpdateTimeEntryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Hours != nil {
		if *req.Hours <= 0 {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Hours must be greater than 0"})
		}
		entry.Hours = *req.Hours
	}
	if req.Description != nil {
		entry.Description = *req.Description
	}
	if req.Date != nil {
		date, err := models.ParseDate(*req.Date)
		if err != nil || date == nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid date"})
		}
		entry.Date = *date
	}

	if err := database.GetDB().Save(&entry).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating time entry"})
	}

	users := loadUsersByIDs([]uint{entry.UserID})
	invoicedPeriods := invoicedPeriodsForProject(taskProjectID(entry.TaskID))
	return c.JSON(buildTimeEntryDTO(entry, users, invoicedPeriods))
}

// DeleteTimeEntry - DELETE /api/v1/projects/:id/time-entries/:entryId
func (h *TaskTimeEntryHandler) DeleteTimeEntry(c *fiber.Ctx) error {
	entryID, err := strconv.Atoi(c.Params("entryId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid time entry ID"})
	}

	if err := database.GetDB().Delete(&models.TaskTimeEntry{}, entryID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting time entry"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Time entry deleted successfully"})
}
