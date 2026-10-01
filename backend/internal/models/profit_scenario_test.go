// backend/internal/models/profit_scenario_test.go
package models

import "testing"

func TestProfitParameterSetBeforeSaveReplacesNilLists(t *testing.T) {
	s := &ProfitParameterSet{}
	if err := s.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if s.PercentItems == nil || len(s.PercentItems) != 0 {
		t.Fatalf("PercentItems: %#v", s.PercentItems)
	}
	if s.FixedMonthlyCosts == nil || len(s.FixedMonthlyCosts) != 0 {
		t.Fatalf("FixedMonthlyCosts: %#v", s.FixedMonthlyCosts)
	}
}

func TestProfitParameterSetBeforeSaveKeepsData(t *testing.T) {
	s := &ProfitParameterSet{
		PercentItems:      JSONList[ProfitPercentItem]{{Label: "a", Percent: 1, Base: "revenue"}},
		FixedMonthlyCosts: JSONList[ProfitFixedCost]{{Label: "b", Amount: 2}},
	}
	if err := s.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if len(s.PercentItems) != 1 || s.PercentItems[0].Label != "a" || len(s.FixedMonthlyCosts) != 1 || s.FixedMonthlyCosts[0].Amount != 2 {
		t.Fatalf("data changed: %+v", s)
	}
}

func TestProfitScenarioBeforeSaveReplacesNilLists(t *testing.T) {
	s := &ProfitScenario{}
	if err := s.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if s.ClientAdjustments == nil || len(s.ClientAdjustments) != 0 {
		t.Fatalf("ClientAdjustments: %#v", s.ClientAdjustments)
	}
	if s.NewClients == nil || len(s.NewClients) != 0 {
		t.Fatalf("NewClients: %#v", s.NewClients)
	}
}

func TestProfitScenarioBeforeSaveKeepsData(t *testing.T) {
	s := &ProfitScenario{
		ClientAdjustments: JSONList[ProfitScenarioAdjustment]{{ClientID: 1, HoursDelta: 5}},
		NewClients:        JSONList[ProfitScenarioNewClient]{{Name: "x", MonthlyRevenue: 10}},
	}
	if err := s.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if len(s.ClientAdjustments) != 1 || s.ClientAdjustments[0].HoursDelta != 5 || len(s.NewClients) != 1 || s.NewClients[0].Name != "x" {
		t.Fatalf("data changed: %+v", s)
	}
}
