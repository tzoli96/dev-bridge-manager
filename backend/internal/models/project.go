package models

import (
	"time"
)

type Project struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name" gorm:"not null" validate:"required,min=1,max=255"`
	Description   string    `json:"description" gorm:"type:text"`
	Status        string    `json:"status" gorm:"default:active" validate:"oneof=active completed on-hold cancelled"`
	CreatedBy     uint      `json:"created_by" gorm:"not null"`
	CreatedByName string    `json:"created_by_name" gorm:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Kapcsolat a User modellel
	Creator User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

type ProjectCreateRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=255"`
	Description string `json:"description"`
	Status      string `json:"status" validate:"omitempty,oneof=active completed on-hold cancelled"`
}

type ProjectUpdateRequest struct {
	Name        string `json:"name" validate:"omitempty,min=1,max=255"`
	Description string `json:"description"`
	Status      string `json:"status" validate:"omitempty,oneof=active completed on-hold cancelled"`
}

type ProjectResponse struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	CreatedBy     uint      `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProjectListResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Project  *ProjectResponse  `json:"project,omitempty"`
	Projects []ProjectResponse `json:"projects,omitempty"`
	Count    int               `json:"count,omitempty"`
}
