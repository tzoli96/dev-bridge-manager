package models

import (
	"time"
)

type ProjectClient struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ProjectID  uint      `json:"project_id" gorm:"not null"`
	ClientID   uint      `json:"client_id" gorm:"not null"`
	AssignedAt time.Time `json:"assigned_at" gorm:"default:now()"`
	AssignedBy uint      `json:"assigned_by"`

	Project        Project `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Client         Client  `json:"client,omitempty" gorm:"foreignKey:ClientID"`
	AssignedByUser User    `json:"assigned_by_user,omitempty" gorm:"foreignKey:AssignedBy"`
}

type ProjectClientCreateRequest struct {
	ClientID uint `json:"client_id" validate:"required"`
}

type ProjectClientResponse struct {
	ID             uint      `json:"id"`
	ProjectID      uint      `json:"project_id"`
	ClientID       uint      `json:"client_id"`
	ClientName     string    `json:"client_name"`
	ClientType     string    `json:"client_type"`
	AssignedAt     time.Time `json:"assigned_at"`
	AssignedBy     uint      `json:"assigned_by"`
	AssignedByName string    `json:"assigned_by_name"`
}

type ProjectClientListResponse struct {
	Success        bool                     `json:"success"`
	Message        string                   `json:"message"`
	ProjectClient  *ProjectClientResponse   `json:"project_client,omitempty"`
	ProjectClients []ProjectClientResponse  `json:"project_clients,omitempty"`
	Count          int                      `json:"count,omitempty"`
}
