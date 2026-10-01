// backend/internal/services/profitability_forecast.go
package services

import (
	"fmt"
	"sort"
	"time"
)

// forecastBaselineMonths is the look-back window the run-rate is averaged
// over; it is also the minimum history before the forecast stops being
// flagged "low data".
const forecastBaselineMonths = 3

// ForecastInvoice is one invoice-month bucket. Fixed marks one-off
// fixed-price invoices, which are not a run-rate and are excluded.
type ForecastInvoice struct {
	ProjectID uint
	ClientID  uint
	Month     string // "YYYY-MM"
	Amount    float64
	Fixed     bool
}

type ForecastInput struct {
	BaselineMonths   []string          // the complete past months averaged over
	Months           []string          // the forecast horizon, "YYYY-MM"
	Invoices         []ForecastInvoice // may include months outside the baseline; ignored
	ContractEndMonth map[uint]string   // project id -> "YYYY-MM" of contract_end_date
	ClientNames      map[uint]string
}

type ForecastClient struct {
	ClientID         uint      `json:"client_id"`
	Name             string    `json:"name"`
	MonthlyAverage   float64   `json:"monthly_average"`
	ContractEndMonth *string   `json:"contract_end_month"`
	MonthsWithData   int       `json:"months_with_data"`
	Committed        []float64 `json:"committed"`
	Dependent        []float64 `json:"dependent"`
}

type Forecast struct {
	BaselineMonths       []string         `json:"baseline_months"`
	Months               []string         `json:"months"`
	Committed            []float64        `json:"committed"`
	Dependent            []float64        `json:"dependent"`
	Clients              []ForecastClient `json:"clients"`
	HistoryMonths        int              `json:"history_months"`
	LowData              bool             `json:"low_data"`
	ExcludedFixedRevenue float64          `json:"excluded_fixed_revenue"`
	Warnings             []string         `json:"warnings"`
}

// FutureMonths returns n "YYYY-MM" labels starting with now's month. The
// current month is included because its invoice is typically issued at the
// start of the next one, so that revenue is still ahead of us.
func FutureMonths(now time.Time, n int) []string {
	if n < 0 {
		n = 0
	}
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	months := make([]string, 0, n)
	for i := 0; i < n; i++ {
		months = append(months, start.AddDate(0, i, 0).Format("2006-01"))
	}
	return months
}

// BuildForecast projects each (project, client) pair's baseline average over
// the horizon. A pair's revenue is "committed" up to and including its
// project's contract end month and "dependent" on renewal afterwards.
// Pairs are processed in sorted order so float sums are deterministic.
func BuildForecast(in ForecastInput) Forecast {
	baseline := make(map[string]bool, len(in.BaselineMonths))
	for _, m := range in.BaselineMonths {
		baseline[m] = true
	}

	type pair struct{ project, client uint }
	sums := map[pair]float64{}
	clientMonths := map[uint]map[string]bool{}
	historyMonths := map[string]bool{}
	var excludedFixed float64

	for _, inv := range in.Invoices {
		if !baseline[inv.Month] {
			continue
		}
		if inv.Fixed {
			excludedFixed += inv.Amount
			continue
		}
		sums[pair{inv.ProjectID, inv.ClientID}] += inv.Amount
		if clientMonths[inv.ClientID] == nil {
			clientMonths[inv.ClientID] = map[string]bool{}
		}
		clientMonths[inv.ClientID][inv.Month] = true
		historyMonths[inv.Month] = true
	}

	pairs := make([]pair, 0, len(sums))
	for p := range sums {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].client != pairs[j].client {
			return pairs[i].client < pairs[j].client
		}
		return pairs[i].project < pairs[j].project
	})

	months := in.Months
	if months == nil {
		months = []string{}
	}
	baselineMonths := in.BaselineMonths
	if baselineMonths == nil {
		baselineMonths = []string{}
	}
	horizon := len(months)

	out := Forecast{
		BaselineMonths:       baselineMonths,
		Months:               months,
		Committed:            make([]float64, horizon),
		Dependent:            make([]float64, horizon),
		Clients:              []ForecastClient{},
		HistoryMonths:        len(historyMonths),
		ExcludedFixedRevenue: excludedFixed,
		Warnings:             []string{},
	}

	rows := map[uint]*ForecastClient{}
	// sums is empty when the baseline is empty, so the division below never
	// runs with a zero denominator.
	denominator := float64(len(in.BaselineMonths))
	for _, p := range pairs {
		avg := sums[p] / denominator

		row, ok := rows[p.client]
		if !ok {
			name := in.ClientNames[p.client]
			if name == "" {
				name = fmt.Sprintf("#%d", p.client)
			}
			row = &ForecastClient{
				ClientID:  p.client,
				Name:      name,
				Committed: make([]float64, horizon),
				Dependent: make([]float64, horizon),
			}
			rows[p.client] = row
		}
		row.MonthlyAverage += avg

		endMonth, hasEnd := in.ContractEndMonth[p.project]
		if hasEnd && (row.ContractEndMonth == nil || endMonth < *row.ContractEndMonth) {
			end := endMonth
			row.ContractEndMonth = &end
		}

		for i, m := range months {
			if hasEnd && m > endMonth { // "YYYY-MM" compares correctly as a string
				row.Dependent[i] += avg
				out.Dependent[i] += avg
			} else {
				row.Committed[i] += avg
				out.Committed[i] += avg
			}
		}
	}

	for id, row := range rows {
		row.MonthsWithData = len(clientMonths[id])
		out.Clients = append(out.Clients, *row)
	}
	sort.Slice(out.Clients, func(i, j int) bool {
		if out.Clients[i].MonthlyAverage != out.Clients[j].MonthlyAverage {
			return out.Clients[i].MonthlyAverage > out.Clients[j].MonthlyAverage
		}
		return out.Clients[i].ClientID < out.Clients[j].ClientID
	})

	out.LowData = out.HistoryMonths < forecastBaselineMonths
	if out.LowData {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"Csak %d hónapnyi rendszeres számlázási előzmény van, az előrejelzés tájékoztató jellegű", out.HistoryMonths))
	}
	if excludedFixed > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"%.0f Ft fix áras bevétel nem része az előrejelzésnek (egyszeri számla, nem rendszeres)", excludedFixed))
	}
	return out
}
