// backend/internal/models/project_password.go
package models

import "time"

type ProjectPassword struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	ProjectID         uint      `json:"project_id" gorm:"not null;index"`
	Title             string    `json:"title" gorm:"not null" validate:"required,min=1,max=255"`
	Username          string    `json:"username"`
	EncryptedPassword string    `json:"-" gorm:"column:encrypted_password;not null"`
	URL               string    `json:"url"`
	Notes             string    `json:"notes"`
	CreatedBy         uint      `json:"created_by" gorm:"not null"`
	UpdatedBy         uint      `json:"updated_by" gorm:"not null"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProjectPasswordCreateRequest struct {
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Username string `json:"username"`
	Password string `json:"password" validate:"required"`
	URL      string `json:"url"`
	Notes    string `json:"notes"`
}

type ProjectPasswordUpdateRequest struct {
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Username string `json:"username"`
	Password string `json:"password" validate:"required"`
	URL      string `json:"url"`
	Notes    string `json:"notes"`
}

// ProjectPasswordDTO is what the API actually returns: the password is
// decrypted server-side and included in the JSON body (the whole point
// of the feature is that a member can retrieve the real value), but it
// never lives on the ProjectPassword struct's exported JSON tag, so a
// stray `db.Find(&passwords)` handler elsewhere in the codebase can
// never leak it by accident - only buildPasswordDTO's explicit decrypt
// path can.
type ProjectPasswordDTO struct {
	ID            uint      `json:"id"`
	ProjectID     uint      `json:"project_id"`
	Title         string    `json:"title"`
	Username      string    `json:"username"`
	Password      string    `json:"password"`
	URL           string    `json:"url"`
	Notes         string    `json:"notes"`
	CreatedBy     uint      `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
