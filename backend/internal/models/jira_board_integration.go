// backend/internal/models/jira_board_integration.go
package models

import "time"

// JiraBoardIntegration links a single kanban board to a Jira Cloud
// project/account. At most one row exists per board (BoardID is unique) -
// connecting a board that's already connected reconnects it in place rather
// than creating a second row.
type JiraBoardIntegration struct {
	ID            uint       `gorm:"primaryKey"`
	BoardID       uint       `gorm:"column:board_id;not null;unique"`
	BaseURL       string     `gorm:"column:base_url;size:255"`
	Email         string     `gorm:"column:email;size:255"`
	APIToken      string     `json:"-" gorm:"column:api_token;size:255"`
	ProjectKey    string     `gorm:"column:project_key;size:50"`
	ConnectedBy   uint       `gorm:"column:connected_by"`
	ConnectedAt   time.Time  `gorm:"column:connected_at"`
	LastSyncAt    *time.Time `gorm:"column:last_sync_at"`
	LastSyncError string     `gorm:"column:last_sync_error"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (JiraBoardIntegration) TableName() string { return "jira_board_integrations" }

// JiraIntegrationStatusDTO is the read-only shape returned by GET .../jira-integration.
type JiraIntegrationStatusDTO struct {
	Connected     bool   `json:"connected"`
	BaseURL       string `json:"baseUrl,omitempty"`
	Email         string `json:"email,omitempty"`
	ProjectKey    string `json:"projectKey,omitempty"`
	LastSyncAt    string `json:"lastSyncAt,omitempty"`
	LastSyncError string `json:"lastSyncError,omitempty"`
}

// ConnectJiraIntegrationRequest is the body of POST .../jira-integration.
type ConnectJiraIntegrationRequest struct {
	BaseURL    string `json:"baseUrl" validate:"required"`
	Email      string `json:"email" validate:"required"`
	APIToken   string `json:"apiToken" validate:"required"`
	ProjectKey string `json:"projectKey" validate:"required"`
}
