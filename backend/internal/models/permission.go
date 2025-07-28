package models

import (
	"time"
)

type Permission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	DisplayName string    `gorm:"size:150;not null" json:"display_name"`
	Description string    `gorm:"type:text" json:"description"`
	Resource    string    `gorm:"size:50;not null" json:"resource"`
	Action      string    `gorm:"size:50;not null" json:"action"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Roles []Role `gorm:"many2many:role_permissions;" json:"roles,omitempty"`
}

// TableName override
func (Permission) TableName() string {
	return "permissions"
}
