// backend/internal/handlers/activity_digest_handler.go
package handlers

import (
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

const activityDigestListLimit = 5

type ActivityDigestHandler struct{}

func NewActivityDigestHandler() *ActivityDigestHandler {
	return &ActivityDigestHandler{}
}

// digestDateRange resolves the [start, end) calendar-day window a digest
// covers: an explicit "YYYY-MM-DD" dateParam if given (empty string keeps
// the day it defaults to), otherwise the calendar day before now. Pure
// function so it can be unit-tested without a database.
func digestDateRange(now time.Time, dateParam string) (time.Time, time.Time, error) {
	if dateParam == "" {
		yesterday := now.AddDate(0, 0, -1)
		start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 0, 1), nil
	}
	start, err := time.ParseInLocation("2006-01-02", dateParam, now.Location())
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, start.AddDate(0, 0, 1), nil
}

// GetDigest - GET /api/v1/activity-digest?date=YYYY-MM-DD - Napi
// tevékenység összefoglaló egy adott naptári napra (alapértelmezésben a
// tegnapi napra), mindenkinek aki be van jelentkezve - ugyanaz a szabály,
// mint a /search végpontnál.
func (h *ActivityDigestHandler) GetDigest(c *fiber.Ctx) error {
	start, end, err := digestDateRange(time.Now(), c.Query("date"))
	if err != nil {
		return c.Status(400).JSON(models.ActivityDigestResponse{Success: false})
	}

	db := database.GetDB()
	resp := models.ActivityDigestResponse{Success: true}

	var newTasks []models.ActivityDigestTask
	db.Table("tasks").
		Select("tasks.id as id, tasks.title as title, projects.name as project_name").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Where("tasks.created_at >= ? AND tasks.created_at < ? AND tasks.is_archived = false", start, end).
		Order("tasks.created_at ASC").
		Scan(&newTasks)
	var newTasksCount int64
	db.Table("tasks").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Where("tasks.created_at >= ? AND tasks.created_at < ? AND tasks.is_archived = false", start, end).
		Count(&newTasksCount)
	resp.NewTasksCount = int(newTasksCount)
	resp.NewTasks = limitDigestTasks(newTasks)

	completedTasksWhere := "task_activity_log.event_type = ? AND task_activity_log.created_at >= ? AND task_activity_log.created_at < ? AND kanban_columns.is_done = true AND tasks.is_archived = false"
	var completedTasks []models.ActivityDigestTask
	db.Table("task_activity_log").
		Select("tasks.id as id, tasks.title as title, projects.name as project_name").
		Joins("JOIN tasks ON tasks.id = task_activity_log.task_id").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Joins("JOIN task_placements ON task_placements.task_id = tasks.id").
		Joins("JOIN kanban_columns ON kanban_columns.id = task_placements.column_id").
		Where(completedTasksWhere, "moved", start, end).
		Order("task_activity_log.created_at ASC").
		Scan(&completedTasks)
	var completedTasksCount int64
	db.Table("task_activity_log").
		Joins("JOIN tasks ON tasks.id = task_activity_log.task_id").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Joins("JOIN task_placements ON task_placements.task_id = tasks.id").
		Joins("JOIN kanban_columns ON kanban_columns.id = task_placements.column_id").
		Where(completedTasksWhere, "moved", start, end).
		Count(&completedTasksCount)
	resp.CompletedTasksCount = int(completedTasksCount)
	resp.CompletedTasks = limitDigestTasks(completedTasks)

	var invoiceStats struct {
		Count int
		Total float64
	}
	db.Table("invoices").
		Select("COUNT(*) as count, COALESCE(SUM(amount), 0) as total").
		Where("created_at >= ? AND created_at < ? AND status = 'created'", start, end).
		Scan(&invoiceStats)
	resp.NewInvoicesCount = invoiceStats.Count
	resp.NewInvoicesTotal = invoiceStats.Total

	var clientEmailsCount int64
	db.Table("emails").
		Where("received_at >= ? AND received_at < ? AND client_id IS NOT NULL", start, end).
		Count(&clientEmailsCount)
	resp.NewClientEmailsCount = int(clientEmailsCount)

	var stallFlagsCount int64
	db.Table("kanban_stall_flags").
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&stallFlagsCount)
	resp.NewStallFlagsCount = int(stallFlagsCount)

	return c.JSON(resp)
}

func limitDigestTasks(tasks []models.ActivityDigestTask) []models.ActivityDigestTask {
	if len(tasks) > activityDigestListLimit {
		return tasks[:activityDigestListLimit]
	}
	return tasks
}
