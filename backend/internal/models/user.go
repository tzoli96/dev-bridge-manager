package models

import (
	"golang.org/x/crypto/bcrypt"
	"time"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Email     string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Position  string    `gorm:"size:100" json:"position"`
	RoleID    uint      `gorm:"not null;index" json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Role Role `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

// TableName override
func (User) TableName() string {
	return "users"
}

// HashPassword hashes the user password
func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword compares password with hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// HasPermission checks if user has specific permission
func (u *User) HasPermission(permissionName string) bool {
	return u.Role.HasPermission(permissionName)
}

// GetPermissions returns user's permissions
func (u *User) GetPermissions() []string {
	return u.Role.GetPermissionNames()
}

// HasRole checks if user has specific role
func (u *User) HasRole(roleName string) bool {
	return u.Role.Name == roleName
}
