// backend/internal/services/profitability_forecast_test.go
package services

import (
	"testing"
	"time"
)

var (
	fcBaseline = []string{"2026-07", "2026-08", "2026-09"}
	fcHorizon  = []string{"2026-10", "2026-11", "2026-12"}
)

func sameFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !approx(a[i], b[i]) {
			return false
		}
	}
	return true
}

// recurring returns one invoice per baseline month for a project/client.
func recurring(project, client uint, amount float64) []ForecastInvoice {
	out := make([]ForecastInvoice, 0, len(fcBaseline))
	for _, m := range fcBaseline {
		out = append(out, ForecastInvoice{ProjectID: project, ClientID: client, Month: m, Amount: amount})
	}
	return out
}

func TestFutureMonthsStartsAtCurrentMonthAndCrossesYear(t *testing.T) {
	now := time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC)
	got := FutureMonths(now, 3)
	want := []string{"2026-11", "2026-12", "2027-01"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestFutureMonthsZeroIsEmptyNotNil(t *testing.T) {
	if got := FutureMonths(time.Now(), 0); got == nil || len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestBuildForecastUsesBaselineAverage(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:    recurring(7, 1, 300000),
		ClientNames: map[uint]string{1: "PIXEL"},
	})
	if len(out.Clients) != 1 {
		t.Fatalf("clients = %d", len(out.Clients))
	}
	c := out.Clients[0]
	if c.Name != "PIXEL" || !approx(c.MonthlyAverage, 300000) || c.MonthsWithData != 3 {
		t.Fatalf("client = %+v", c)
	}
	if !sameFloats(out.Committed, []float64{300000, 300000, 300000}) || !sameFloats(out.Dependent, []float64{0, 0, 0}) {
		t.Fatalf("totals committed=%v dependent=%v", out.Committed, out.Dependent)
	}
	if out.LowData || len(out.Warnings) != 0 {
		t.Fatalf("unexpected low data / warnings: %v", out.Warnings)
	}
}

func TestBuildForecastDividesByBaselineLengthNotByActiveMonths(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices: []ForecastInvoice{{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 90000}},
	})
	if !approx(out.Clients[0].MonthlyAverage, 30000) {
		t.Fatalf("avg = %v, want 30000", out.Clients[0].MonthlyAverage)
	}
}

func TestBuildForecastSplitsAfterContractEndMonth(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         recurring(7, 1, 300000),
		ContractEndMonth: map[uint]string{7: "2026-11"},
	})
	// End month itself is still committed; only months after it depend on renewal.
	if !sameFloats(out.Committed, []float64{300000, 300000, 0}) || !sameFloats(out.Dependent, []float64{0, 0, 300000}) {
		t.Fatalf("committed=%v dependent=%v", out.Committed, out.Dependent)
	}
	c := out.Clients[0]
	if c.ContractEndMonth == nil || *c.ContractEndMonth != "2026-11" {
		t.Fatalf("contract end = %v", c.ContractEndMonth)
	}
}

func TestBuildForecastContractEndedBeforeHorizonIsAllDependent(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         recurring(7, 1, 300000),
		ContractEndMonth: map[uint]string{7: "2026-08"},
	})
	if !sameFloats(out.Committed, []float64{0, 0, 0}) || !sameFloats(out.Dependent, []float64{300000, 300000, 300000}) {
		t.Fatalf("committed=%v dependent=%v", out.Committed, out.Dependent)
	}
}

func TestBuildForecastAppliesContractEndPerProject(t *testing.T) {
	invoices := append(recurring(7, 1, 300000), recurring(8, 1, 150000)...)
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         invoices,
		ContractEndMonth: map[uint]string{7: "2026-10"}, // project 8 has no end
	})
	if len(out.Clients) != 1 {
		t.Fatalf("clients = %d", len(out.Clients))
	}
	c := out.Clients[0]
	if !approx(c.MonthlyAverage, 450000) {
		t.Fatalf("avg = %v", c.MonthlyAverage)
	}
	if !sameFloats(c.Committed, []float64{450000, 150000, 150000}) || !sameFloats(c.Dependent, []float64{0, 300000, 300000}) {
		t.Fatalf("committed=%v dependent=%v", c.Committed, c.Dependent)
	}
	if c.ContractEndMonth == nil || *c.ContractEndMonth != "2026-10" {
		t.Fatalf("contract end = %v", c.ContractEndMonth)
	}
}

func TestBuildForecastExcludesFixedPriceInvoicesAndWarns(t *testing.T) {
	invoices := append(recurring(7, 1, 100000),
		ForecastInvoice{ProjectID: 9, ClientID: 2, Month: "2026-09", Amount: 600000, Fixed: true})
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: invoices})
	if len(out.Clients) != 1 || out.Clients[0].ClientID != 1 {
		t.Fatalf("clients = %+v", out.Clients)
	}
	if !approx(out.ExcludedFixedRevenue, 600000) {
		t.Fatalf("excluded = %v", out.ExcludedFixedRevenue)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestBuildForecastFlagsLowData(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices: []ForecastInvoice{
			{ProjectID: 7, ClientID: 1, Month: "2026-08", Amount: 100000},
			{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 100000},
		},
	})
	if !out.LowData || out.HistoryMonths != 2 || len(out.Warnings) != 1 {
		t.Fatalf("lowData=%v history=%d warnings=%v", out.LowData, out.HistoryMonths, out.Warnings)
	}
}

func TestBuildForecastIgnoresInvoicesOutsideBaseline(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices: []ForecastInvoice{{ProjectID: 7, ClientID: 1, Month: "2026-05", Amount: 999999}},
	})
	if len(out.Clients) != 0 {
		t.Fatalf("clients = %+v", out.Clients)
	}
}

func TestBuildForecastEmptyInputHasNonNilSlices(t *testing.T) {
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon})
	if out.Clients == nil || out.Warnings == nil || out.Committed == nil || out.Dependent == nil {
		t.Fatalf("nil slice in %+v", out)
	}
	if len(out.Committed) != 3 || !out.LowData {
		t.Fatalf("committed=%v lowData=%v", out.Committed, out.LowData)
	}
}

func TestBuildForecastSortsClientsByAverageThenID(t *testing.T) {
	invoices := append(recurring(7, 2, 100000), recurring(8, 1, 100000)...)
	invoices = append(invoices, recurring(9, 3, 500000)...)
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: invoices})
	got := []uint{out.Clients[0].ClientID, out.Clients[1].ClientID, out.Clients[2].ClientID}
	want := []uint{3, 1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestBuildForecastUnknownClientNameFallsBackToID(t *testing.T) {
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: recurring(7, 42, 1000)})
	if out.Clients[0].Name != "#42" {
		t.Fatalf("name = %q", out.Clients[0].Name)
	}
}

func TestFutureMonthsNegativeIsEmpty(t *testing.T) {
	got := FutureMonths(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), -1)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestBuildForecastShowsEarliestContractEndAcrossProjects(t *testing.T) {
	invoices := append(recurring(7, 1, 100000), recurring(8, 1, 100000)...)
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         invoices,
		ContractEndMonth: map[uint]string{7: "2026-12", 8: "2026-10"},
	})
	c := out.Clients[0]
	if c.ContractEndMonth == nil || *c.ContractEndMonth != "2026-10" {
		t.Fatalf("contract end = %v, want 2026-10", c.ContractEndMonth)
	}
}

func TestBuildForecastFixedOnlyMonthDoesNotCountAsHistory(t *testing.T) {
	invoices := []ForecastInvoice{
		{ProjectID: 7, ClientID: 1, Month: "2026-07", Amount: 100000},
		{ProjectID: 7, ClientID: 1, Month: "2026-08", Amount: 100000},
		{ProjectID: 9, ClientID: 1, Month: "2026-09", Amount: 600000, Fixed: true}, // no recurring revenue in 2026-09
	}
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: invoices})
	if out.HistoryMonths != 2 || !out.LowData {
		t.Fatalf("history=%d lowData=%v, want 2/true", out.HistoryMonths, out.LowData)
	}
	if out.Clients[0].MonthsWithData != 2 {
		t.Fatalf("months with data = %d, want 2", out.Clients[0].MonthsWithData)
	}
}

func TestBuildForecastContractEndingInLastHorizonMonthStaysCommitted(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         recurring(7, 1, 300000),
		ContractEndMonth: map[uint]string{7: "2026-12"}, // last month of fcHorizon
	})
	// The end month itself is still committed, so nothing depends on renewal inside the horizon.
	if !sameFloats(out.Committed, []float64{300000, 300000, 300000}) || !sameFloats(out.Dependent, []float64{0, 0, 0}) {
		t.Fatalf("committed=%v dependent=%v", out.Committed, out.Dependent)
	}
}
