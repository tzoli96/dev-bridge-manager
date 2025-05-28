package models

import (
	"time"
)

type Role struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:50;uniqueIndex;not null" json:"name"`
	DisplayName string    `gorm:"size:100;not null" json:"display_name"`
	Description string    `gorm:"type:text" json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	Users       []User       `gorm:"foreignKey:RoleID" json:"users,omitempty"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

// TableName override
func (Role) TableName() string {
	return "roles"
}

// HasPermission checks if role has specific permission
func (r *Role) HasPermission(permissionName string) bool {
	for _, permission := range r.Permissions {
		if permission.Name == permissionName && permission.IsActive {
			return true
		}
	}
	return false
}

// GetPermissionNames returns slice of permission names
func (r *Role) GetPermissionNames() []string {
	var names []string
	for _, permission := range r.Permissions {
		if permission.IsActive {
			names = append(names, permission.Name)
		}
	}
	return names
}
