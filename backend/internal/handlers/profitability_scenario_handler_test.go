// backend/internal/handlers/profitability_scenario_handler_test.go
package handlers

import (
	"math"
	"strings"
	"testing"

	"dev-bridge-manager/internal/models"
)

func validSet() models.ProfitParameterSetRequest {
	return models.ProfitParameterSetRequest{
		Name: "Alap",
		PercentItems: []models.ProfitPercentItem{
			{Label: "Járulék", Percent: 10, Base: "revenue"},
			{Label: "Adó", Percent: 15, Base: "after_costs"},
		},
		FixedMonthlyCosts: []models.ProfitFixedCost{{Label: "Eszközök", Amount: 20000}},
	}
}

func TestValidateParameterSetRequestAcceptsValid(t *testing.T) {
	req := validSet()
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatalf("valid set rejected: %s", msg)
	}
}

func TestValidateParameterSetRequestAcceptsEmptyLists(t *testing.T) {
	req := models.ProfitParameterSetRequest{Name: "Üres"}
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatalf("empty lists rejected: %s", msg)
	}
}

func TestValidateParameterSetRequestTrims(t *testing.T) {
	req := validSet()
	req.Name = "  Alap  "
	req.PercentItems[0].Label = "  Járulék "
	req.FixedMonthlyCosts[0].Label = " Eszközök  "
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatal(msg)
	}
	if req.Name != "Alap" || req.PercentItems[0].Label != "Járulék" || req.FixedMonthlyCosts[0].Label != "Eszközök" {
		t.Fatalf("not trimmed: %+v", req)
	}
}

func TestValidateParameterSetRequestRejects(t *testing.T) {
	cases := map[string]func(r *models.ProfitParameterSetRequest){
		"blank name":          func(r *models.ProfitParameterSetRequest) { r.Name = "   " },
		"name too long":       func(r *models.ProfitParameterSetRequest) { r.Name = strings.Repeat("a", 256) },
		"blank percent label": func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Label = " " },
		"label too long":      func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Label = strings.Repeat("a", 101) },
		"negative percent":    func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Percent = -1 },
		"percent above 100":   func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Percent = 100.5 },
		"unknown base":        func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Base = "profit" },
		"blank cost label":    func(r *models.ProfitParameterSetRequest) { r.FixedMonthlyCosts[0].Label = "" },
		"negative cost":       func(r *models.ProfitParameterSetRequest) { r.FixedMonthlyCosts[0].Amount = -1 },
		"huge cost":           func(r *models.ProfitParameterSetRequest) { r.FixedMonthlyCosts[0].Amount = 2_000_000_000 },
		"too many percents": func(r *models.ProfitParameterSetRequest) {
			r.PercentItems = make([]models.ProfitPercentItem, 21)
			for i := range r.PercentItems {
				r.PercentItems[i] = models.ProfitPercentItem{Label: "x", Base: "revenue"}
			}
		},
		"too many costs": func(r *models.ProfitParameterSetRequest) {
			r.FixedMonthlyCosts = make([]models.ProfitFixedCost, 21)
			for i := range r.FixedMonthlyCosts {
				r.FixedMonthlyCosts[i] = models.ProfitFixedCost{Label: "x"}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validSet()
			mutate(&req)
			if msg := validateParameterSetRequest(&req); msg == "" {
				t.Fatalf("accepted: %+v", req)
			}
		})
	}
}

func TestValidateParameterSetRequestAcceptsBoundaries(t *testing.T) {
	req := validSet()
	req.PercentItems[0].Percent = 0
	req.PercentItems[1].Percent = 100
	req.FixedMonthlyCosts[0].Amount = 1_000_000_000
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatalf("boundaries rejected: %s", msg)
	}
}

func validScenario() models.ProfitScenarioRequest {
	rate := 18000.0
	return models.ProfitScenarioRequest{
		Name: "Áremelés", HorizonMonths: 6, ParameterSetID: 1, CapacityHoursPerMonth: 120,
		ClientAdjustments: []models.ProfitScenarioAdjustment{
			{ClientID: 1, NewHourlyRate: &rate, HoursDelta: 5},
			{ClientID: 2, Drops: true},
		},
		NewClients: []models.ProfitScenarioNewClient{{Name: "Új Kft", MonthlyRevenue: 100000, MonthlyHours: 20}},
	}
}

func TestValidateScenarioRequestAcceptsValid(t *testing.T) {
	req := validScenario()
	if msg := validateScenarioRequest(&req, true); msg != "" {
		t.Fatalf("valid scenario rejected: %s", msg)
	}
}

func TestValidateScenarioRequestNameOnlyRequiredWhenAsked(t *testing.T) {
	req := validScenario()
	req.Name = ""
	if msg := validateScenarioRequest(&req, true); msg == "" {
		t.Fatal("blank name accepted for save")
	}
	if msg := validateScenarioRequest(&req, false); msg != "" {
		t.Fatalf("blank name rejected for compute: %s", msg)
	}
}

func TestValidateScenarioRequestTrims(t *testing.T) {
	req := validScenario()
	req.Name = "  Áremelés  "
	req.NewClients[0].Name = " Új Kft "
	if msg := validateScenarioRequest(&req, true); msg != "" {
		t.Fatal(msg)
	}
	if req.Name != "Áremelés" || req.NewClients[0].Name != "Új Kft" {
		t.Fatalf("not trimmed: %+v", req)
	}
}

func TestValidateScenarioRequestRejects(t *testing.T) {
	big := 2_000_000_000.0
	neg := -1.0
	rate := 100.0
	cases := map[string]func(r *models.ProfitScenarioRequest){
		"name too long":        func(r *models.ProfitScenarioRequest) { r.Name = strings.Repeat("a", 256) },
		"horizon too short":    func(r *models.ProfitScenarioRequest) { r.HorizonMonths = 2 },
		"horizon too long":     func(r *models.ProfitScenarioRequest) { r.HorizonMonths = 7 },
		"no parameter set":     func(r *models.ProfitScenarioRequest) { r.ParameterSetID = 0 },
		"zero capacity":        func(r *models.ProfitScenarioRequest) { r.CapacityHoursPerMonth = 0 },
		"negative capacity":    func(r *models.ProfitScenarioRequest) { r.CapacityHoursPerMonth = -5 },
		"capacity above month": func(r *models.ProfitScenarioRequest) { r.CapacityHoursPerMonth = 745 },
		"adjustment no client": func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].ClientID = 0 },
		"duplicate client": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments[1].ClientID = r.ClientAdjustments[0].ClientID
		},
		"rate and fixed both": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments[0].NewFixedPrice = &rate
		},
		"negative rate": func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].NewHourlyRate = &neg },
		"huge fixed price": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments[0].NewHourlyRate = nil
			r.ClientAdjustments[0].NewFixedPrice = &big
		},
		"hours delta too low":         func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].HoursDelta = -745 },
		"hours delta too high":        func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].HoursDelta = 745 },
		"new client no name":          func(r *models.ProfitScenarioRequest) { r.NewClients[0].Name = "  " },
		"new client negative revenue": func(r *models.ProfitScenarioRequest) { r.NewClients[0].MonthlyRevenue = -1 },
		"new client hours too high":   func(r *models.ProfitScenarioRequest) { r.NewClients[0].MonthlyHours = 745 },
		"too many adjustments": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments = make([]models.ProfitScenarioAdjustment, 201)
			for i := range r.ClientAdjustments {
				r.ClientAdjustments[i] = models.ProfitScenarioAdjustment{ClientID: uint(i + 1)}
			}
		},
		"too many new clients": func(r *models.ProfitScenarioRequest) {
			r.NewClients = make([]models.ProfitScenarioNewClient, 51)
			for i := range r.NewClients {
				r.NewClients[i] = models.ProfitScenarioNewClient{Name: "x"}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validScenario()
			mutate(&req)
			if msg := validateScenarioRequest(&req, true); msg == "" {
				t.Fatalf("accepted: %+v", req)
			}
		})
	}
}

func TestValidateScenarioRequestAcceptsBoundaries(t *testing.T) {
	req := validScenario()
	req.HorizonMonths = 3
	req.CapacityHoursPerMonth = 744
	req.ClientAdjustments[0].HoursDelta = -744
	req.NewClients[0].MonthlyHours = 744
	if msg := validateScenarioRequest(&req, true); msg != "" {
		t.Fatalf("boundaries rejected: %s", msg)
	}
}

func TestValidateParameterSetRequestRejectsNonFinite(t *testing.T) {
	for vname, v := range map[string]float64{"NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1)} {
		t.Run("percent "+vname, func(t *testing.T) {
			req := validSet()
			req.PercentItems[0].Percent = v
			if msg := validateParameterSetRequest(&req); msg == "" {
				t.Fatalf("accepted %v", v)
			}
		})
		t.Run("amount "+vname, func(t *testing.T) {
			req := validSet()
			req.FixedMonthlyCosts[0].Amount = v
			if msg := validateParameterSetRequest(&req); msg == "" {
				t.Fatalf("accepted %v", v)
			}
		})
	}
}

func TestValidateScenarioRequestRejectsNonFinite(t *testing.T) {
	fields := map[string]func(r *models.ProfitScenarioRequest, v float64){
		"capacity":    func(r *models.ProfitScenarioRequest, v float64) { r.CapacityHoursPerMonth = v },
		"hourly rate": func(r *models.ProfitScenarioRequest, v float64) { r.ClientAdjustments[0].NewHourlyRate = &v },
		"fixed price": func(r *models.ProfitScenarioRequest, v float64) {
			r.ClientAdjustments[0].NewHourlyRate = nil
			r.ClientAdjustments[0].NewFixedPrice = &v
		},
		"hours delta": func(r *models.ProfitScenarioRequest, v float64) { r.ClientAdjustments[0].HoursDelta = v },
		"new revenue": func(r *models.ProfitScenarioRequest, v float64) { r.NewClients[0].MonthlyRevenue = v },
		"new hours":   func(r *models.ProfitScenarioRequest, v float64) { r.NewClients[0].MonthlyHours = v },
	}
	for fname, set := range fields {
		for vname, v := range map[string]float64{"NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1)} {
			t.Run(fname+" "+vname, func(t *testing.T) {
				req := validScenario()
				set(&req, v)
				if msg := validateScenarioRequest(&req, true); msg == "" {
					t.Fatalf("accepted %v", v)
				}
			})
		}
	}
}
