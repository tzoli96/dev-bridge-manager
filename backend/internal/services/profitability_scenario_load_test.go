// backend/internal/services/profitability_scenario_load_test.go
package services

import "testing"

func TestBuildScenarioBase_ClientInBoth(t *testing.T) {
	f := Forecast{
		BaselineMonths: []string{"2026-07", "2026-08", "2026-09"},
		LowData:        true,
		Warnings:       []string{"w"},
		Clients:        []ForecastClient{{ClientID: 1, Name: "A", MonthlyAverage: 300000}},
	}
	o := Overview{Clients: []RateRow{{ID: 1, LoggedHours: 30, EmailHours: 6, MeetingHours: 3}}}
	b := buildScenarioBase(f, o)
	if len(b.Clients) != 1 {
		t.Fatalf("clients = %d", len(b.Clients))
	}
	c := b.Clients[0]
	if c.ClientID != 1 || c.Name != "A" || !approx(c.MonthlyRevenue, 300000) ||
		!approx(c.LoggedHours, 10) || !approx(c.OverheadHours, 3) {
		t.Fatalf("client = %+v", c)
	}
	if len(b.BaselineMonths) != 3 || !b.LowData || len(b.Warnings) != 1 {
		t.Fatalf("meta = %+v", b)
	}
}

func TestBuildScenarioBase_ClientMissingFromOverview(t *testing.T) {
	f := Forecast{Clients: []ForecastClient{{ClientID: 2, Name: "B", MonthlyAverage: 100000}}}
	b := buildScenarioBase(f, Overview{Clients: []RateRow{{ID: 9, LoggedHours: 99}}})
	if len(b.Clients) != 1 {
		t.Fatalf("clients = %d", len(b.Clients))
	}
	c := b.Clients[0]
	if !approx(c.MonthlyRevenue, 100000) || c.LoggedHours != 0 || c.OverheadHours != 0 {
		t.Fatalf("client = %+v", c)
	}
}

func TestBuildScenarioBase_OverviewOnlyClientDropped(t *testing.T) {
	f := Forecast{Clients: []ForecastClient{{ClientID: 1, MonthlyAverage: 1}}}
	o := Overview{Clients: []RateRow{{ID: 1}, {ID: 5, LoggedHours: 40}}}
	b := buildScenarioBase(f, o)
	if len(b.Clients) != 1 || b.Clients[0].ClientID != 1 {
		t.Fatalf("clients = %+v", b.Clients)
	}
}

func TestBuildScenarioBase_EmptyForecastNonNil(t *testing.T) {
	b := buildScenarioBase(Forecast{LowData: true}, Overview{})
	if b.Clients == nil || len(b.Clients) != 0 {
		t.Fatalf("clients = %#v", b.Clients)
	}
	if !b.LowData {
		t.Fatal("LowData not propagated")
	}
}
