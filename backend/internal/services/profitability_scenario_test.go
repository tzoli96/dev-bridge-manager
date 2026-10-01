// backend/internal/services/profitability_scenario_test.go
package services

import (
	"math"
	"testing"

	"dev-bridge-manager/internal/models"
)

func f64(v float64) *float64 { return &v }

// Two clients: c1 300000/month, 20 logged + 4 overhead hours; c2 100000/month,
// 10 logged + 2 overhead hours. Baseline: revenue 400000, hours 36.
func scBase() ScenarioBase {
	return ScenarioBase{
		BaselineMonths: []string{"2026-07", "2026-08", "2026-09"},
		Clients: []ScenarioClientBase{
			{ClientID: 1, Name: "PIXEL", MonthlyRevenue: 300000, LoggedHours: 20, OverheadHours: 4},
			{ClientID: 2, Name: "ACME", MonthlyRevenue: 100000, LoggedHours: 10, OverheadHours: 2},
		},
	}
}

// 20000 fixed cost, 10% of revenue, 15% of the result after costs.
func scParams() models.ProfitParameterSet {
	return models.ProfitParameterSet{
		Name:              "Alap",
		FixedMonthlyCosts: models.JSONList[models.ProfitFixedCost]{{Label: "Eszközök", Amount: 20000}},
		PercentItems: models.JSONList[models.ProfitPercentItem]{
			{Label: "Járulék", Percent: 10, Base: PercentBaseRevenue},
			{Label: "Adó", Percent: 15, Base: PercentBaseAfterCosts},
		},
	}
}

func scInput(adj ...models.ProfitScenarioAdjustment) ScenarioInput {
	return ScenarioInput{
		Months:        []string{"2026-10", "2026-11", "2026-12"},
		CapacityHours: 60,
		Parameters:    scParams(),
		Adjustments:   adj,
	}
}

func TestBuildScenarioBaselineNumbers(t *testing.T) {
	m := BuildScenario(scBase(), scInput()).Baseline
	if !approx(m.Revenue, 400000) || !approx(m.FixedCosts, 20000) || !approx(m.ResultAfterCosts, 340000) || !approx(m.NetProfit, 289000) {
		t.Fatalf("baseline = %+v", m)
	}
	if len(m.RevenueItems) != 1 || !approx(m.RevenueItems[0].Amount, 40000) {
		t.Fatalf("revenue items = %+v", m.RevenueItems)
	}
	if len(m.AfterCostsItems) != 1 || !approx(m.AfterCostsItems[0].Amount, 51000) {
		t.Fatalf("after-costs items = %+v", m.AfterCostsItems)
	}
	if !approx(m.RequiredHours, 36) || m.Utilization == nil || !approx(*m.Utilization, 60) || m.Overloaded {
		t.Fatalf("hours/utilization = %v %v %v", m.RequiredHours, m.Utilization, m.Overloaded)
	}
	if m.NetProfitPerCapacityHour == nil || !approx(*m.NetProfitPerCapacityHour, 289000.0/60.0) {
		t.Fatalf("per capacity hour = %v", m.NetProfitPerCapacityHour)
	}
}

func TestBuildScenarioWithoutChangesEqualsBaseline(t *testing.T) {
	out := BuildScenario(scBase(), scInput())
	if !approx(out.Monthly.NetProfit, out.Baseline.NetProfit) || !approx(out.Monthly.Revenue, out.Baseline.Revenue) {
		t.Fatalf("monthly %+v != baseline %+v", out.Monthly, out.Baseline)
	}
}

func TestBuildScenarioNewHourlyRate(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 1, NewHourlyRate: f64(18000)}))
	m := out.Monthly
	// c1: 18000 x 20 = 360000; total 460000; 46000 + 59100 on top of 20000 fixed.
	if !approx(m.Revenue, 460000) || !approx(m.ResultAfterCosts, 394000) || !approx(m.NetProfit, 334900) || !approx(m.RequiredHours, 36) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioDropClient(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 2, Drops: true}))
	m := out.Monthly
	if !approx(m.Revenue, 300000) || !approx(m.NetProfit, 212500) || !approx(m.RequiredHours, 24) {
		t.Fatalf("monthly = %+v", m)
	}
	if !out.Clients[1].Dropped || !approx(out.Clients[1].Revenue, 0) || !approx(out.Clients[1].Hours, 0) {
		t.Fatalf("client row = %+v", out.Clients[1])
	}
}

func TestBuildScenarioHoursDeltaScalesRevenueAtCurrentRate(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 1, HoursDelta: 10}))
	m := out.Monthly
	// c1 logged 20 -> 30 at 15000/h = 450000; hours 30+4 = 34 (+12 for c2).
	if !approx(m.Revenue, 550000) || !approx(m.NetProfit, 403750) || !approx(m.RequiredHours, 46) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioNewFixedPrice(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 2, NewFixedPrice: f64(150000)}))
	m := out.Monthly
	if !approx(m.Revenue, 450000) || !approx(m.RequiredHours, 36) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioNegativeLoggedHoursClampToZero(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 1, NewHourlyRate: f64(18000), HoursDelta: -30}))
	row := out.Clients[0]
	if !approx(row.Revenue, 0) || !approx(row.Hours, 4) {
		t.Fatalf("client row = %+v", row)
	}
}

func TestBuildScenarioNewClientIsMarkedAndOverloadsCapacity(t *testing.T) {
	in := scInput()
	in.CapacityHours = 50
	in.NewClients = []models.ProfitScenarioNewClient{{Name: "Új Kft", MonthlyRevenue: 100000, MonthlyHours: 20}}
	out := BuildScenario(scBase(), in)
	if !approx(out.Monthly.Revenue, 500000) || !approx(out.Monthly.RequiredHours, 56) || !out.Monthly.Overloaded {
		t.Fatalf("monthly = %+v", out.Monthly)
	}
	last := out.Clients[len(out.Clients)-1]
	if !last.IsNew || last.Name != "Új Kft" || last.ClientID != 0 {
		t.Fatalf("new client row = %+v", last)
	}
	if out.Baseline.Overloaded {
		t.Fatalf("baseline (36h of 50) must not be overloaded: %+v", out.Baseline)
	}
}

func TestBuildScenarioNegativeResultHasNoAfterCostsAmount(t *testing.T) {
	base := ScenarioBase{Clients: []ScenarioClientBase{{ClientID: 1, Name: "A", MonthlyRevenue: 10000, LoggedHours: 10}}}
	in := scInput()
	in.CapacityHours = 100
	out := BuildScenario(base, in)
	m := out.Monthly
	// 10000 - 20000 fixed - 1000 (10%) = -11000; the 15% item must not go negative.
	if !approx(m.ResultAfterCosts, -11000) || !approx(m.AfterCostsItems[0].Amount, 0) || !approx(m.NetProfit, -11000) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioEmptyParametersNetEqualsRevenue(t *testing.T) {
	in := scInput()
	in.Parameters = models.ProfitParameterSet{Name: "Üres"}
	m := BuildScenario(scBase(), in).Monthly
	if !approx(m.NetProfit, m.Revenue) || len(m.RevenueItems) != 0 || len(m.AfterCostsItems) != 0 {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioZeroCapacityHasNoPerHourMetrics(t *testing.T) {
	in := scInput()
	in.CapacityHours = 0
	m := BuildScenario(scBase(), in).Monthly
	if m.Utilization != nil || m.NetProfitPerCapacityHour != nil || m.Overloaded {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioUnknownClientAdjustmentIsSkippedWithWarning(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 99, Drops: true}))
	if !approx(out.Monthly.NetProfit, out.Baseline.NetProfit) {
		t.Fatalf("unknown client changed the result: %+v", out.Monthly)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestBuildScenarioHorizonTotalsMultiplyByMonths(t *testing.T) {
	out := BuildScenario(scBase(), scInput())
	if !approx(out.Horizon.NetProfit, 289000*3) || !approx(out.Horizon.Revenue, 400000*3) || !approx(out.BaselineHorizon.NetProfit, 289000*3) {
		t.Fatalf("horizon = %+v baseline = %+v", out.Horizon, out.BaselineHorizon)
	}
}

func TestBuildScenarioBasisReportsWhatWasUsed(t *testing.T) {
	out := BuildScenario(scBase(), scInput())
	b := out.Basis
	if b.ParameterSetName != "Alap" || len(b.PercentItems) != 2 || len(b.FixedMonthlyCosts) != 1 || !approx(b.CapacityHours, 60) || b.HorizonMonths != 3 || len(b.BaselineMonths) != 3 {
		t.Fatalf("basis = %+v", b)
	}
}

func TestBuildScenarioPassesThroughBaseWarningsAndLowData(t *testing.T) {
	base := scBase()
	base.LowData = true
	base.Warnings = []string{"kevés adat"}
	out := BuildScenario(base, scInput())
	if !out.LowData || len(out.Warnings) != 1 || out.Warnings[0] != "kevés adat" {
		t.Fatalf("lowData=%v warnings=%v", out.LowData, out.Warnings)
	}
}

func TestBuildScenarioEmptyBaseHasNonNilSlices(t *testing.T) {
	out := BuildScenario(ScenarioBase{}, ScenarioInput{Months: []string{"2026-10"}})
	if out.Clients == nil || out.Warnings == nil || out.Monthly.RevenueItems == nil || out.Monthly.AfterCostsItems == nil || out.Basis.PercentItems == nil || out.Basis.FixedMonthlyCosts == nil || out.Basis.BaselineMonths == nil {
		t.Fatalf("nil slice in %+v", out)
	}
}

func TestBuildScenarioTwoAfterCostsItemsDoNotCompound(t *testing.T) {
	in := scInput()
	in.Parameters.PercentItems = append(in.Parameters.PercentItems, models.ProfitPercentItem{Label: "B", Percent: 10, Base: PercentBaseAfterCosts})
	m := BuildScenario(scBase(), in).Monthly
	if len(m.AfterCostsItems) != 2 {
		t.Fatalf("after-costs items = %+v", m.AfterCostsItems)
	}
	if !approx(m.AfterCostsItems[0].Amount, 51000) || !approx(m.AfterCostsItems[1].Amount, 34000) {
		t.Fatalf("after-costs amounts = %+v", m.AfterCostsItems)
	}
	if !approx(m.NetProfit, 255000) {
		t.Fatalf("net profit = %v", m.NetProfit)
	}
}

func TestBuildScenarioZeroLoggedHoursWithoutPriceKeepsBaseRevenue(t *testing.T) {
	base := ScenarioBase{Clients: []ScenarioClientBase{{ClientID: 1, Name: "Retainer", MonthlyRevenue: 50000, LoggedHours: 0, OverheadHours: 2}}}
	out := BuildScenario(base, scInput(models.ProfitScenarioAdjustment{ClientID: 1, HoursDelta: 5}))
	c := out.Clients[0]
	if !approx(c.Revenue, 50000) || !approx(c.Hours, 7) {
		t.Fatalf("client = %+v", c)
	}
	for _, v := range []float64{c.Revenue, c.Hours, out.Monthly.NetProfit} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("non-finite value %v in %+v", v, out)
		}
	}
}

func TestBuildScenarioEmptyInputHasNonNilMonths(t *testing.T) {
	out := BuildScenario(ScenarioBase{}, ScenarioInput{})
	if out.Months == nil || len(out.Months) != 0 {
		t.Fatalf("months = %#v", out.Months)
	}
	if out.Clients == nil || out.Warnings == nil || out.Basis.BaselineMonths == nil {
		t.Fatalf("nil slice in %+v", out)
	}
}
