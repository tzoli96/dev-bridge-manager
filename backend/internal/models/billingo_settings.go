// backend/internal/models/billingo_settings.go
package models

import "time"

type BillingoSettings struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	APIKey    string    `json:"-" gorm:"column:api_key;size:255"`
	BlockID   string    `json:"block_id" gorm:"size:50"`
	UpdatedBy uint      `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BillingoSettings) TableName() string { return "billingo_settings" }

type BillingoSettingsUpdateRequest struct {
	APIKey  string `json:"api_key"`
	BlockID string `json:"block_id"`
}

type BillingoSettingsResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	APIKeyMasked string    `json:"api_key_masked"`
	BlockID      string    `json:"block_id"`
	UpdatedBy    uint      `json:"updated_by"`
	UpdatedAt    time.Time `json:"updated_at"`
}
