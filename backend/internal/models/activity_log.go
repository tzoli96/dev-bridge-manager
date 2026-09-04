package models

import "time"

type TaskActivityLog struct {
	ID        uint `gorm:"primaryKey"`
	TaskID    uint `gorm:"not null;index"`
	UserID    uint `gorm:"not null"`
	EventType string
	FieldName *string
	OldValue  *string `gorm:"type:text"`
	NewValue  *string `gorm:"type:text"`
	CreatedAt time.Time
}

func (TaskActivityLog) TableName() string { return "task_activity_log" }

type ActivityLogDTO struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"taskId"`
	UserID    string          `json:"userId"`
	User      *TaskUserRefDTO `json:"user,omitempty"`
	EventType string          `json:"eventType"`
	FieldName string          `json:"fieldName,omitempty"`
	OldValue  string          `json:"oldValue,omitempty"`
	NewValue  string          `json:"newValue,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}
