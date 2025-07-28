package models

import (
	"time"
)

type ProjectAssignment struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ProjectID  uint      `json:"project_id" gorm:"not null"`
	UserID     uint      `json:"user_id" gorm:"not null"`
	Role       string    `json:"role" gorm:"default:member" validate:"oneof=owner manager member viewer"`
	AssignedAt time.Time `json:"assigned_at" gorm:"default:now()"`
	AssignedBy uint      `json:"assigned_by"`
	IsActive   bool      `json:"is_active" gorm:"default:true"`

	Project        Project `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	User           User    `json:"user,omitempty" gorm:"foreignKey:UserID"`
	AssignedByUser User    `json:"assigned_by_user,omitempty" gorm:"foreignKey:AssignedBy"`
}

type ProjectAssignmentCreateRequest struct {
	UserID uint   `json:"user_id" validate:"required"`
	Role   string `json:"role" validate:"omitempty,oneof=owner manager member viewer"`
}

type ProjectAssignmentUpdateRequest struct {
	Role string `json:"role" validate:"required,oneof=owner manager member viewer"`
}

type ProjectAssignmentResponse struct {
	ID             uint      `json:"id"`
	ProjectID      uint      `json:"project_id"`
	UserID         uint      `json:"user_id"`
	UserName       string    `json:"user_name"`
	UserEmail      string    `json:"user_email"`
	Role           string    `json:"role"`
	AssignedAt     time.Time `json:"assigned_at"`
	AssignedBy     uint      `json:"assigned_by"`
	AssignedByName string    `json:"assigned_by_name"`
	IsActive       bool      `json:"is_active"`
}

type ProjectAssignmentListResponse struct {
	Success     bool                        `json:"success"`
	Message     string                      `json:"message"`
	Assignment  *ProjectAssignmentResponse  `json:"assignment,omitempty"`
	Assignments []ProjectAssignmentResponse `json:"assignments,omitempty"`
	Count       int                         `json:"count,omitempty"`
}
