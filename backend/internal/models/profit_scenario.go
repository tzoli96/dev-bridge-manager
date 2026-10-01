// backend/internal/models/profit_scenario.go
package models

import (
	"time"

	"gorm.io/gorm"
)

// ProfitPercentItem is a percentage line of a parameter set. Base is
// "revenue" (percent of revenue) or "after_costs" (percent of the result
// after fixed costs and revenue-based items). No rates are built in; the
// user supplies every value.
type ProfitPercentItem struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Base    string  `json:"base"`
}

type ProfitFixedCost struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
}

type ProfitParameterSet struct {
	ID                uint                        `json:"id" gorm:"primaryKey"`
	Name              string                      `json:"name" gorm:"not null"`
	PercentItems      JSONList[ProfitPercentItem] `json:"percent_items" gorm:"type:jsonb;not null"`
	FixedMonthlyCosts JSONList[ProfitFixedCost]   `json:"fixed_monthly_costs" gorm:"type:jsonb;not null"`
	CreatedBy         uint                        `json:"created_by" gorm:"not null"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}

func (ProfitParameterSet) TableName() string { return "profit_parameter_sets" }

type ProfitParameterSetRequest struct {
	Name              string              `json:"name"`
	PercentItems      []ProfitPercentItem `json:"percent_items"`
	FixedMonthlyCosts []ProfitFixedCost   `json:"fixed_monthly_costs"`
}

// ProfitScenarioAdjustment changes one existing client. At most one of
// NewHourlyRate / NewFixedPrice may be set; Drops removes the client.
type ProfitScenarioAdjustment struct {
	ClientID      uint     `json:"client_id"`
	NewHourlyRate *float64 `json:"new_hourly_rate"`
	NewFixedPrice *float64 `json:"new_fixed_price"`
	HoursDelta    float64  `json:"hours_delta"`
	Drops         bool     `json:"drops"`
}

// ProfitScenarioNewClient is a hypothetical client; MonthlyHours is total hours.
type ProfitScenarioNewClient struct {
	Name           string  `json:"name"`
	MonthlyRevenue float64 `json:"monthly_revenue"`
	MonthlyHours   float64 `json:"monthly_hours"`
}

type ProfitScenario struct {
	ID                    uint                               `json:"id" gorm:"primaryKey"`
	Name                  string                             `json:"name" gorm:"not null"`
	HorizonMonths         int                                `json:"horizon_months" gorm:"not null"`
	ParameterSetID        uint                               `json:"parameter_set_id" gorm:"not null;index"`
	CapacityHoursPerMonth float64                            `json:"capacity_hours_per_month" gorm:"not null"`
	ClientAdjustments     JSONList[ProfitScenarioAdjustment] `json:"client_adjustments" gorm:"type:jsonb;not null"`
	NewClients            JSONList[ProfitScenarioNewClient]  `json:"new_clients" gorm:"type:jsonb;not null"`
	CreatedBy             uint                               `json:"created_by" gorm:"not null"`
	CreatedAt             time.Time                          `json:"created_at"`
	UpdatedAt             time.Time                          `json:"updated_at"`
}

func (ProfitScenario) TableName() string { return "profit_scenarios" }

type ProfitScenarioRequest struct {
	Name                  string                     `json:"name"`
	HorizonMonths         int                        `json:"horizon_months"`
	ParameterSetID        uint                       `json:"parameter_set_id"`
	CapacityHoursPerMonth float64                    `json:"capacity_hours_per_month"`
	ClientAdjustments     []ProfitScenarioAdjustment `json:"client_adjustments"`
	NewClients            []ProfitScenarioNewClient  `json:"new_clients"`
}

// BeforeSave keeps the jsonb columns NOT NULL: pgx encodes a nil slice as SQL
// NULL without calling JSONList.Value, so nil lists are replaced by empty ones.
func (s *ProfitParameterSet) BeforeSave(tx *gorm.DB) error {
	if s.PercentItems == nil {
		s.PercentItems = JSONList[ProfitPercentItem]{}
	}
	if s.FixedMonthlyCosts == nil {
		s.FixedMonthlyCosts = JSONList[ProfitFixedCost]{}
	}
	return nil
}

// BeforeSave: see ProfitParameterSet.BeforeSave.
func (s *ProfitScenario) BeforeSave(tx *gorm.DB) error {
	if s.ClientAdjustments == nil {
		s.ClientAdjustments = JSONList[ProfitScenarioAdjustment]{}
	}
	if s.NewClients == nil {
		s.NewClients = JSONList[ProfitScenarioNewClient]{}
	}
	return nil
}
