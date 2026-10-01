// backend/internal/services/profitability_scenario_load.go
package services

import "time"

// LoadScenarioBase builds the baseline run-rate the scenarios modify. Revenue
// per client is the forecast's monthly average (recurring invoices only);
// hours come from the overview over the same last-3-complete-months window,
// averaged per month. Clients without recurring revenue are not part of the
// base (a fixed-price client can be modelled as a new client). A client that
// has BOTH recurring and fixed-price projects only counts the recurring revenue
// while its logged hours include the fixed-price projects' hours, so its base
// understates revenue per hour.
func LoadScenarioBase() (ScenarioBase, error) {
	// The horizon argument is irrelevant here: only the forecast's baseline
	// (clients, monthly averages, warnings) is used.
	forecast, err := LoadForecast(forecastBaselineMonths)
	if err != nil {
		return ScenarioBase{}, err
	}
	overview, err := LoadOverview(forecastBaselineMonths)
	if err != nil {
		return ScenarioBase{}, err
	}
	return buildScenarioBase(forecast, overview), nil
}

// buildScenarioBase joins the forecast clients with the overview rows (by
// client ID) and averages the overview hours per baseline month. A forecast
// client absent from the overview gets zero hours.
func buildScenarioBase(forecast Forecast, overview Overview) ScenarioBase {
	rows := make(map[uint]RateRow, len(overview.Clients))
	for _, r := range overview.Clients {
		rows[r.ID] = r
	}

	months := float64(forecastBaselineMonths)
	clients := make([]ScenarioClientBase, 0, len(forecast.Clients))
	for _, c := range forecast.Clients {
		row := rows[c.ClientID] // zero value when the client has no hours/mail in the window
		clients = append(clients, ScenarioClientBase{
			ClientID:       c.ClientID,
			Name:           c.Name,
			MonthlyRevenue: c.MonthlyAverage,
			LoggedHours:    row.LoggedHours / months,
			OverheadHours:  (row.EmailHours + row.MeetingHours) / months,
		})
	}

	return ScenarioBase{
		BaselineMonths: forecast.BaselineMonths,
		Clients:        clients,
		LowData:        forecast.LowData,
		Warnings:       forecast.Warnings,
	}
}

// ComputeScenario loads the baseline and prices the scenario over
// horizonMonths months starting with the current one. Every call reloads the
// baseline (no cache).
func ComputeScenario(in ScenarioInput, horizonMonths int) (ScenarioResult, error) {
	base, err := LoadScenarioBase()
	if err != nil {
		return ScenarioResult{}, err
	}
	in.Months = FutureMonths(time.Now(), horizonMonths)
	return BuildScenario(base, in), nil
}
