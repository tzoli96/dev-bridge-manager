// backend/internal/services/profitability_scenario.go
package services

import (
	"fmt"
	"math"

	"dev-bridge-manager/internal/models"
)

const (
	PercentBaseRevenue    = "revenue"
	PercentBaseAfterCosts = "after_costs"
)

// ScenarioClientBase is one client's monthly run-rate: revenue from the
// forecast baseline, hours from the overview (logged hours, plus the
// estimated e-mail and meeting hours as overhead).
type ScenarioClientBase struct {
	ClientID       uint
	Name           string
	MonthlyRevenue float64
	LoggedHours    float64
	OverheadHours  float64
}

type ScenarioBase struct {
	BaselineMonths []string
	Clients        []ScenarioClientBase
	LowData        bool
	Warnings       []string
}

type ScenarioInput struct {
	Months        []string // horizon labels; the monthly result is flat across them
	CapacityHours float64
	Parameters    models.ProfitParameterSet
	Adjustments   []models.ProfitScenarioAdjustment
	NewClients    []models.ProfitScenarioNewClient
}

type ScenarioItemAmount struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Base    string  `json:"base"`
	Amount  float64 `json:"amount"`
}

type ScenarioMonth struct {
	Revenue                  float64              `json:"revenue"`
	FixedCosts               float64              `json:"fixed_costs"`
	RevenueItems             []ScenarioItemAmount `json:"revenue_items"`
	ResultAfterCosts         float64              `json:"result_after_costs"`
	AfterCostsItems          []ScenarioItemAmount `json:"after_costs_items"`
	NetProfit                float64              `json:"net_profit"`
	RequiredHours            float64              `json:"required_hours"`
	CapacityHours            float64              `json:"capacity_hours"`
	Utilization              *float64             `json:"utilization"`
	Overloaded               bool                 `json:"overloaded"`
	NetProfitPerCapacityHour *float64             `json:"net_profit_per_capacity_hour"`
}

type ScenarioClientResult struct {
	ClientID    uint    `json:"client_id"`
	Name        string  `json:"name"`
	IsNew       bool    `json:"is_new"`
	Dropped     bool    `json:"dropped"`
	BaseRevenue float64 `json:"base_revenue"`
	BaseHours   float64 `json:"base_hours"`
	Revenue     float64 `json:"revenue"`
	Hours       float64 `json:"hours"`
}

type ScenarioTotals struct {
	Revenue   float64 `json:"revenue"`
	NetProfit float64 `json:"net_profit"`
}

// ScenarioBasis lists exactly what the numbers were computed with, so a
// result can be checked by hand.
type ScenarioBasis struct {
	ParameterSetName  string                     `json:"parameter_set_name"`
	PercentItems      []models.ProfitPercentItem `json:"percent_items"`
	FixedMonthlyCosts []models.ProfitFixedCost   `json:"fixed_monthly_costs"`
	CapacityHours     float64                    `json:"capacity_hours"`
	BaselineMonths    []string                   `json:"baseline_months"`
	HorizonMonths     int                        `json:"horizon_months"`
}

type ScenarioResult struct {
	Months          []string               `json:"months"`
	Monthly         ScenarioMonth          `json:"monthly"`
	Baseline        ScenarioMonth          `json:"baseline"`
	Horizon         ScenarioTotals         `json:"horizon"`
	BaselineHorizon ScenarioTotals         `json:"baseline_horizon"`
	Clients         []ScenarioClientResult `json:"clients"`
	LowData         bool                   `json:"low_data"`
	Warnings        []string               `json:"warnings"`
	Basis           ScenarioBasis          `json:"basis"`
}

// BuildScenario applies the scenario to the baseline run-rate and prices it
// with the parameter set. Clients are processed in base order so the float
// sums are deterministic.
func BuildScenario(base ScenarioBase, in ScenarioInput) ScenarioResult {
	adjustments := make(map[uint]models.ProfitScenarioAdjustment, len(in.Adjustments))
	for _, a := range in.Adjustments {
		adjustments[a.ClientID] = a
	}
	known := make(map[uint]bool, len(base.Clients))

	clients := make([]ScenarioClientResult, 0, len(base.Clients)+len(in.NewClients))
	var revenue, hours, baseRevenue, baseHours float64

	for _, c := range base.Clients {
		known[c.ClientID] = true
		bRevenue := c.MonthlyRevenue
		bHours := c.LoggedHours + c.OverheadHours
		baseRevenue += bRevenue
		baseHours += bHours

		row := ScenarioClientResult{
			ClientID: c.ClientID, Name: c.Name,
			BaseRevenue: bRevenue, BaseHours: bHours,
			Revenue: bRevenue, Hours: bHours,
		}
		if a, ok := adjustments[c.ClientID]; ok {
			if a.Drops {
				row.Revenue, row.Hours, row.Dropped = 0, 0, true
			} else {
				logged := math.Max(c.LoggedHours+a.HoursDelta, 0)
				switch {
				case a.NewHourlyRate != nil:
					row.Revenue = *a.NewHourlyRate * logged
				case a.NewFixedPrice != nil:
					row.Revenue = *a.NewFixedPrice
				case c.LoggedHours > 0:
					row.Revenue = c.MonthlyRevenue * logged / c.LoggedHours
				}
				row.Hours = logged + c.OverheadHours
			}
		}
		revenue += row.Revenue
		hours += row.Hours
		clients = append(clients, row)
	}

	warnings := append([]string{}, base.Warnings...)
	for _, a := range in.Adjustments {
		if !known[a.ClientID] {
			warnings = append(warnings, fmt.Sprintf(
				"A(z) #%d ügyfélre vonatkozó módosítás kimaradt: nincs rendszeres bevétele az alapidőszakban", a.ClientID))
		}
	}

	for _, n := range in.NewClients {
		clients = append(clients, ScenarioClientResult{
			Name: n.Name, IsNew: true, Revenue: n.MonthlyRevenue, Hours: n.MonthlyHours,
		})
		revenue += n.MonthlyRevenue
		hours += n.MonthlyHours
	}

	monthly := buildScenarioMonth(revenue, hours, in.Parameters, in.CapacityHours)
	baseline := buildScenarioMonth(baseRevenue, baseHours, in.Parameters, in.CapacityHours)

	horizon := float64(len(in.Months))
	months := in.Months
	if months == nil {
		months = []string{}
	}
	baselineMonths := base.BaselineMonths
	if baselineMonths == nil {
		baselineMonths = []string{}
	}
	percentItems := []models.ProfitPercentItem(in.Parameters.PercentItems)
	if percentItems == nil {
		percentItems = []models.ProfitPercentItem{}
	}
	fixedCosts := []models.ProfitFixedCost(in.Parameters.FixedMonthlyCosts)
	if fixedCosts == nil {
		fixedCosts = []models.ProfitFixedCost{}
	}

	return ScenarioResult{
		Months:          months,
		Monthly:         monthly,
		Baseline:        baseline,
		Horizon:         ScenarioTotals{Revenue: monthly.Revenue * horizon, NetProfit: monthly.NetProfit * horizon},
		BaselineHorizon: ScenarioTotals{Revenue: baseline.Revenue * horizon, NetProfit: baseline.NetProfit * horizon},
		Clients:         clients,
		LowData:         base.LowData,
		Warnings:        warnings,
		Basis: ScenarioBasis{
			ParameterSetName:  in.Parameters.Name,
			PercentItems:      percentItems,
			FixedMonthlyCosts: fixedCosts,
			CapacityHours:     in.CapacityHours,
			BaselineMonths:    baselineMonths,
			HorizonMonths:     len(in.Months),
		},
	}
}

// buildScenarioMonth prices one month: revenue-based items come off first,
// the remainder after fixed costs is the base of the after_costs items (which
// do not compound and never go negative), the rest is net profit.
func buildScenarioMonth(revenue, hours float64, p models.ProfitParameterSet, capacity float64) ScenarioMonth {
	m := ScenarioMonth{
		Revenue:         revenue,
		RequiredHours:   hours,
		CapacityHours:   capacity,
		RevenueItems:    []ScenarioItemAmount{},
		AfterCostsItems: []ScenarioItemAmount{},
	}
	for _, c := range p.FixedMonthlyCosts {
		m.FixedCosts += c.Amount
	}

	var revenueItemsTotal float64
	for _, it := range p.PercentItems {
		if it.Base != PercentBaseRevenue {
			continue
		}
		amount := revenue * it.Percent / 100
		revenueItemsTotal += amount
		m.RevenueItems = append(m.RevenueItems, ScenarioItemAmount{Label: it.Label, Percent: it.Percent, Base: it.Base, Amount: amount})
	}

	m.ResultAfterCosts = revenue - m.FixedCosts - revenueItemsTotal

	afterBase := math.Max(m.ResultAfterCosts, 0)
	var afterTotal float64
	for _, it := range p.PercentItems {
		if it.Base != PercentBaseAfterCosts {
			continue
		}
		amount := afterBase * it.Percent / 100
		afterTotal += amount
		m.AfterCostsItems = append(m.AfterCostsItems, ScenarioItemAmount{Label: it.Label, Percent: it.Percent, Base: it.Base, Amount: amount})
	}

	m.NetProfit = m.ResultAfterCosts - afterTotal

	if capacity > 0 {
		utilization := hours / capacity * 100
		m.Utilization = &utilization
		m.Overloaded = hours > capacity+1e-9
		perHour := m.NetProfit / capacity
		m.NetProfitPerCapacityHour = &perHour
	}
	return m
}
