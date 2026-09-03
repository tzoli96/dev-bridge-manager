package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ---------- DB models ----------

type KanbanColumn struct {
	ID        uint `gorm:"primaryKey"`
	BoardID   uint `gorm:"not null;index" json:"boardId"`
	Title     string
	Color     string
	Position  int
	MaxTasks  *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (KanbanColumn) TableName() string { return "kanban_columns" }

type Board struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;index" json:"projectId"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Board) TableName() string { return "boards" }

type TaskPlacement struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"not null;index" json:"taskId"`
	BoardID   uint      `gorm:"not null;index" json:"boardId"`
	ColumnID  uint      `gorm:"not null;index" json:"columnId"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TaskPlacement) TableName() string { return "task_placements" }

type Task struct {
	ID              uint `gorm:"primaryKey"`
	ProjectID       uint `gorm:"not null;index"`
	Title           string
	Description     string
	HTMLDescription string
	Priority        string
	Status          string
	AssigneeID      *uint
	EstimatedHours  float64
	Tags            string `gorm:"type:jsonb"`
	DueDate         *time.Time `gorm:"type:date"`
	IsArchived      bool
	CreatedBy       uint
	UpdatedBy       uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Task) TableName() string { return "tasks" }

type TaskComment struct {
	ID          uint `gorm:"primaryKey"`
	TaskID      uint `gorm:"not null;index"`
	Content     string
	HTMLContent string
	UserID      uint
	IsEdited    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TaskComment) TableName() string { return "task_comments" }

type TaskTimeEntry struct {
	ID          uint `gorm:"primaryKey"`
	TaskID      uint `gorm:"not null;index"`
	UserID      uint
	Hours       float64
	Description string
	Date        time.Time `gorm:"type:date"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TaskTimeEntry) TableName() string { return "task_time_entries" }

// ---------- Response DTOs (camelCase / string ids, matching the frontend's kanban types) ----------

type TaskTagDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type TaskAssigneeDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
}

type TaskUserRefDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type TimeEntryDTO struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"taskId"`
	Hours       float64         `json:"hours"`
	Description string          `json:"description"`
	Date        string          `json:"date"`
	UserID      string          `json:"userId"`
	User        *TaskUserRefDTO `json:"user,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type TaskCommentDTO struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"taskId"`
	Content     string          `json:"content"`
	HTMLContent string          `json:"htmlContent,omitempty"`
	UserID      string          `json:"userId"`
	User        *TaskUserRefDTO `json:"user,omitempty"`
	IsEdited    bool            `json:"isEdited"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type TaskDTO struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	HTMLDescription string           `json:"htmlDescription,omitempty"`
	Priority        string           `json:"priority"`
	Status          string           `json:"status"`
	ColumnID        string           `json:"columnId"`
	ProjectID       string           `json:"projectId"`
	AssigneeID      string           `json:"assigneeId,omitempty"`
	Assignee        *TaskAssigneeDTO `json:"assignee,omitempty"`
	EstimatedHours  float64          `json:"estimatedHours"`
	LoggedHours     float64          `json:"loggedHours"`
	Tags            []TaskTagDTO     `json:"tags"`
	TimeEntries     []TimeEntryDTO   `json:"timeEntries"`
	Comments        []TaskCommentDTO `json:"comments"`
	Attachments     []interface{}    `json:"attachments"`
	Position        int              `json:"position"`
	DueDate         string           `json:"dueDate,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	CreatedBy       string           `json:"createdBy"`
	UpdatedBy       string           `json:"updatedBy"`
}

type KanbanColumnDTO struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	MaxTasks  *int      `json:"maxTasks,omitempty"`
	Tasks     []TaskDTO `json:"tasks"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type KanbanSettingsDTO struct {
	EnableWipLimits     bool   `json:"enableWipLimits"`
	EnableTimeTracking  bool   `json:"enableTimeTracking"`
	EnableComments      bool   `json:"enableComments"`
	EnablePriorities    bool   `json:"enablePriorities"`
	EnableTags          bool   `json:"enableTags"`
	DefaultEstimateUnit string `json:"defaultEstimateUnit"`
}

func DefaultKanbanSettings() KanbanSettingsDTO {
	return KanbanSettingsDTO{
		EnableWipLimits:     true,
		EnableTimeTracking:  true,
		EnableComments:      true,
		EnablePriorities:    true,
		EnableTags:          true,
		DefaultEstimateUnit: "hours",
	}
}

type KanbanBoardDTO struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"projectId"`
	Name      string            `json:"name"`
	Columns   []KanbanColumnDTO `json:"columns"`
	Settings  KanbanSettingsDTO `json:"settings"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type BoardDTO struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ---------- Request DTOs ----------

type CreateTaskRequest struct {
	Title           string   `json:"title" validate:"required,min=1,max=255"`
	Description     string   `json:"description"`
	HTMLDescription string   `json:"htmlDescription"`
	Priority        string   `json:"priority" validate:"omitempty,oneof=low medium high urgent"`
	BoardID         string   `json:"boardId" validate:"required"`
	ColumnID        string   `json:"columnId" validate:"required"`
	AssigneeID      string   `json:"assigneeId"`
	EstimatedHours  *float64 `json:"estimatedHours"`
	Tags            []string `json:"tags"`
	DueDate         string   `json:"dueDate"`
}

type UpdateTaskRequest struct {
	Title           *string   `json:"title"`
	Description     *string   `json:"description"`
	HTMLDescription *string   `json:"htmlDescription"`
	Priority        *string   `json:"priority" validate:"omitempty,oneof=low medium high urgent"`
	AssigneeID      *string   `json:"assigneeId"`
	EstimatedHours  *float64  `json:"estimatedHours"`
	Tags            *[]string `json:"tags"`
	DueDate         *string   `json:"dueDate"`
}

type MoveTaskRequest struct {
	ColumnID string `json:"columnId" validate:"required"`
	Position int    `json:"position"`
}

type CreateColumnRequest struct {
	Title    string `json:"title" validate:"required,min=1,max=100"`
	Color    string `json:"color"`
	Position int    `json:"position"`
	MaxTasks *int   `json:"maxTasks"`
}

type UpdateColumnRequest struct {
	Title    *string `json:"title"`
	Color    *string `json:"color"`
	Position *int    `json:"position"`
	MaxTasks *int    `json:"maxTasks"`
}

type CreateBoardRequest struct {
	Name     string `json:"name" validate:"required"`
	Position int    `json:"position"`
}

type UpdateBoardRequest struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}

type PlaceTaskRequest struct {
	ColumnID string `json:"columnId" validate:"required"`
}

type ColumnOrder struct {
	ColumnID string `json:"columnId"`
	Position int    `json:"position"`
}

type ReorderColumnsRequest struct {
	Orders []ColumnOrder `json:"orders"`
}

type CreateCommentRequest struct {
	Content     string `json:"content" validate:"required,min=1"`
	HTMLContent string `json:"htmlContent"`
}

type UpdateCommentRequest struct {
	Content     string `json:"content" validate:"required,min=1"`
	HTMLContent string `json:"htmlContent"`
}

type CreateTimeEntryRequest struct {
	Hours       float64 `json:"hours" validate:"required,gt=0"`
	Description string  `json:"description"`
	Date        string  `json:"date" validate:"required"`
}

type UpdateTimeEntryRequest struct {
	Hours       *float64 `json:"hours"`
	Description *string  `json:"description"`
	Date        *string  `json:"date"`
}

// ---------- Helpers ----------

func IDToStr(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func StrToID(s string) (uint, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

var tagColorPalette = []string{"#3B82F6", "#EF4444", "#10B981", "#F59E0B", "#8B5CF6", "#06B6D4", "#EC4899", "#6B7280"}

func tagColorFor(name string) string {
	sum := 0
	for _, r := range name {
		sum += int(r)
	}
	return tagColorPalette[sum%len(tagColorPalette)]
}

// TagsToJSON converts plain tag names into the JSONB-stored [{name,color}] representation.
func TagsToJSON(names []string) string {
	type tag struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	tags := make([]tag, 0, len(names))
	seen := make(map[string]bool)
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		tags = append(tags, tag{Name: n, Color: tagColorFor(n)})
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// TagsFromJSON parses the JSONB-stored tags into response DTOs.
func TagsFromJSON(raw string) []TaskTagDTO {
	type tag struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	var tags []tag
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &tags)
	}
	dtos := make([]TaskTagDTO, 0, len(tags))
	for _, t := range tags {
		dtos = append(dtos, TaskTagDTO{ID: t.Name, Name: t.Name, Color: t.Color})
	}
	return dtos
}

func FormatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func ParseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		// also accept full RFC3339 timestamps sent by the client
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, err
		}
	}
	return &t, nil
}
