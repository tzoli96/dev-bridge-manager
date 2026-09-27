// backend/internal/services/kanban_stall.go
package services

import (
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

// kanbanStallThreshold matches the approved design: a task sitting in the
// same non-done column for 5 days or more counts as stalled.
const kanbanStallThreshold = 5 * 24 * time.Hour

// kanbanStallRepeatInterval mirrors invoiceReminderRepeatInterval's
// dismiss-then-recheck pattern: a dismissed flag is reconsidered once this
// much time has passed and the task is still stuck in the same column.
const kanbanStallRepeatInterval = 5 * 24 * time.Hour

// kanbanStallCheckInterval mirrors invoiceReminderCheckInterval's daily
// cadence - a task's time-in-column only needs to be re-evaluated once a day.
const kanbanStallCheckInterval = 24 * time.Hour

// taskIsStalled reports whether a task that entered its current column at
// enteredAt has been sitting there long enough to count as stalled. Pure
// function so it can be unit-tested without a database.
func taskIsStalled(enteredAt time.Time, now time.Time) bool {
	return now.Sub(enteredAt) >= kanbanStallThreshold
}

// stallFlagIsDue decides whether a new pending KanbanStallFlag should be
// created for a task, given the most recently created flag for it (nil if
// none exists yet). No flag is proposed while one is already pending, to
// avoid stacking duplicates; a dismissed flag is reconsidered once
// kanbanStallRepeatInterval has elapsed.
func stallFlagIsDue(latest *models.KanbanStallFlag, now time.Time) bool {
	if latest == nil {
		return true
	}
	if latest.Status == "pending" {
		return false
	}
	return now.Sub(latest.CreatedAt) >= kanbanStallRepeatInterval
}

// RunKanbanStallCheck flags every non-archived task that's been sitting in
// the same non-done column for kanbanStallThreshold or longer. A task's
// "entered this column at" time is the most recent "moved" activity log
// entry for it, or its creation time if it has never moved. Flags whose
// column no longer matches the task's current placement (it moved on) are
// cleaned up automatically instead of being left stale.
func RunKanbanStallCheck() {
	db := database.GetDB()
	now := time.Now()

	var pending []models.KanbanStallFlag
	if err := db.Where("status = ?", "pending").Find(&pending).Error; err != nil {
		log.Printf("⚠️ Kanban stall check: failed to load pending flags: %v", err)
		return
	}
	for _, flag := range pending {
		var placement models.TaskPlacement
		if err := db.Where("task_id = ?", flag.TaskID).First(&placement).Error; err != nil {
			continue
		}
		if placement.ColumnID != flag.ColumnID {
			db.Delete(&models.KanbanStallFlag{}, flag.ID)
		}
	}

	var placements []models.TaskPlacement
	if err := db.Find(&placements).Error; err != nil {
		log.Printf("⚠️ Kanban stall check: failed to load task placements: %v", err)
		return
	}

	var doneColumnIDs []uint
	db.Model(&models.KanbanColumn{}).Where("is_done = ?", true).Pluck("id", &doneColumnIDs)
	doneColumns := make(map[uint]bool, len(doneColumnIDs))
	for _, id := range doneColumnIDs {
		doneColumns[id] = true
	}

	for _, placement := range placements {
		if doneColumns[placement.ColumnID] {
			continue
		}
		checkTaskForStall(db, placement, now)
	}
}

func checkTaskForStall(db *gorm.DB, placement models.TaskPlacement, now time.Time) {
	var task models.Task
	if err := db.First(&task, placement.TaskID).Error; err != nil || task.IsArchived {
		return
	}

	enteredAt := task.CreatedAt
	var lastMove models.TaskActivityLog
	if err := db.Where("task_id = ? AND event_type = ?", task.ID, "moved").
		Order("created_at DESC").First(&lastMove).Error; err == nil {
		enteredAt = lastMove.CreatedAt
	}

	if !taskIsStalled(enteredAt, now) {
		return
	}

	var latest models.KanbanStallFlag
	err := db.Where("task_id = ?", task.ID).Order("created_at DESC").First(&latest).Error
	var latestPtr *models.KanbanStallFlag
	if err == nil {
		latestPtr = &latest
	}

	if !stallFlagIsDue(latestPtr, now) {
		return
	}

	flag := models.KanbanStallFlag{
		TaskID:      task.ID,
		ProjectID:   task.ProjectID,
		BoardID:     placement.BoardID,
		ColumnID:    placement.ColumnID,
		DaysStalled: int(now.Sub(enteredAt).Hours() / 24),
		Status:      "pending",
		CreatedAt:   now,
	}
	if err := db.Create(&flag).Error; err != nil {
		log.Printf("⚠️ Kanban stall check: failed to create flag for task %d: %v", task.ID, err)
	}
}

// StartKanbanStallScheduler mirrors StartInvoiceReminderScheduler's plain
// time.Ticker pattern: runs once at startup, then once every 24h.
func StartKanbanStallScheduler() {
	log.Println("🗂️ Kanban stall check: running")
	RunKanbanStallCheck()

	ticker := time.NewTicker(kanbanStallCheckInterval)
	for range ticker.C {
		log.Println("🗂️ Kanban stall check: running")
		RunKanbanStallCheck()
	}
}
