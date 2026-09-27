// backend/internal/models/kanban_stall_flag.go
package models

import "time"

// KanbanStallFlag flags a task that has sat in the same non-done kanban
// column for too long. Created by services.RunKanbanStallCheck, surfaced to
// super_admin users only (see routes/kanban_stall_routes.go). Dismissing a
// flag just snoozes it - RunKanbanStallCheck re-flags the task once
// kanbanStallRepeatInterval has elapsed and it's still stuck in the same
// column. If the task moves to a different column before that, the flag is
// cleaned up automatically instead of being re-flagged.
type KanbanStallFlag struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	TaskID      uint       `json:"task_id" gorm:"not null"`
	ProjectID   uint       `json:"project_id" gorm:"not null"`
	BoardID     uint       `json:"board_id" gorm:"not null"`
	ColumnID    uint       `json:"column_id" gorm:"not null"`
	DaysStalled int        `json:"days_stalled" gorm:"not null"`
	Status      string     `json:"status" gorm:"size:20;not null;default:'pending'"`
	CreatedAt   time.Time  `json:"created_at"`
	DismissedBy *uint      `json:"dismissed_by"`
	DismissedAt *time.Time `json:"dismissed_at"`
}

func (KanbanStallFlag) TableName() string { return "kanban_stall_flags" }

// KanbanStallFlagWithNames adds the display names ListKanbanStallFlags joins
// in, for the dashboard's stalled-tasks widget.
type KanbanStallFlagWithNames struct {
	KanbanStallFlag
	TaskTitle   string `json:"task_title"`
	ProjectName string `json:"project_name"`
	BoardName   string `json:"board_name"`
	ColumnTitle string `json:"column_title"`
}

type KanbanStallFlagListResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message,omitempty"`
	Flags   []KanbanStallFlagWithNames `json:"flags,omitempty"`
}

type KanbanStallFlagActionResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message,omitempty"`
	Flag    *KanbanStallFlag `json:"flag,omitempty"`
}
