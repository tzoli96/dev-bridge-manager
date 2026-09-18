// backend/internal/models/billingo_settings.go
package models

import "time"

type BillingoSettings struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	APIKey               string    `json:"-" gorm:"column:api_key;size:255"`
	BlockID              string    `json:"block_id" gorm:"size:50"`
	DefaultUnit          string    `json:"default_unit" gorm:"column:default_unit;size:50"`
	DefaultUnitPriceType string    `json:"default_unit_price_type" gorm:"column:default_unit_price_type;size:10"`
	UpdatedBy            uint      `json:"updated_by"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (BillingoSettings) TableName() string { return "billingo_settings" }

type BillingoSettingsUpdateRequest struct {
	APIKey               string `json:"api_key"`
	BlockID              string `json:"block_id"`
	DefaultUnit          string `json:"default_unit"`
	DefaultUnitPriceType string `json:"default_unit_price_type"`
}

type BillingoSettingsResponse struct {
	Success              bool      `json:"success"`
	Message              string    `json:"message"`
	APIKeyMasked         string    `json:"api_key_masked"`
	BlockID              string    `json:"block_id"`
	DefaultUnit          string    `json:"default_unit"`
	DefaultUnitPriceType string    `json:"default_unit_price_type"`
	UpdatedBy            uint      `json:"updated_by"`
	UpdatedAt            time.Time `json:"updated_at"`
}
