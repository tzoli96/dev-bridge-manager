// backend/internal/models/profitability.go
package models

import "time"

type ProfitSettings struct {
	ID                           uint      `json:"-" gorm:"primaryKey"`
	MinutesPerInboundEmail       float64   `json:"minutes_per_inbound_email"`
	MinutesPerOutboundEmail      float64   `json:"minutes_per_outbound_email"`
	DefaultCapacityHoursPerMonth float64   `json:"default_capacity_hours_per_month"`
	UnderpricedRatioThreshold    float64   `json:"underpriced_ratio_threshold"`
	UpdatedAt                    time.Time `json:"updated_at"`
}

func (ProfitSettings) TableName() string { return "profit_settings" }

type ClientMeetingAllowance struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	ClientID      uint      `json:"client_id" gorm:"uniqueIndex;not null"`
	HoursPerMonth float64   `json:"hours_per_month"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ClientMeetingAllowance) TableName() string { return "client_meeting_allowances" }

type ProfitSettingsRequest struct {
	MinutesPerInboundEmail       float64 `json:"minutes_per_inbound_email"`
	MinutesPerOutboundEmail      float64 `json:"minutes_per_outbound_email"`
	DefaultCapacityHoursPerMonth float64 `json:"default_capacity_hours_per_month"`
	UnderpricedRatioThreshold    float64 `json:"underpriced_ratio_threshold"`
}

type MeetingAllowanceRequest struct {
	HoursPerMonth float64 `json:"hours_per_month"`
}
