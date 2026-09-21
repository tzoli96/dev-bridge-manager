// backend/internal/services/invoice_calc_test.go
package services

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestCalculateFixedAmount(t *testing.T) {
	got := CalculateFixedAmount(150000)
	if got != 150000 {
		t.Fatalf("expected 150000, got %v", got)
	}
}

func TestCalculateHourlyAmount(t *testing.T) {
	got := CalculateHourlyAmount(12.5, 8000)
	want := 100000.0
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestFormatHUFAmount(t *testing.T) {
	cases := map[float64]string{
		0:       "0",
		850000:  "850 000",
		1234567: "1 234 567",
		999:     "999",
		-25000:  "-25 000",
		12345.6: "12 346",
	}
	for amount, want := range cases {
		if got := formatHUFAmount(amount); got != want {
			t.Fatalf("formatHUFAmount(%v): expected %q, got %q", amount, want, got)
		}
	}
}

func TestFormatHoursHU(t *testing.T) {
	cases := map[float64]string{
		12:   "12",
		12.5: "12.5",
		0:    "0",
	}
	for hours, want := range cases {
		if got := formatHoursHU(hours); got != want {
			t.Fatalf("formatHoursHU(%v): expected %q, got %q", hours, want, got)
		}
	}
}

func TestBuildInvoiceDraftSummaryFixedPrice(t *testing.T) {
	fixedPrice := 500000.0
	project := models.Project{Name: "Céges weboldal", PricingType: "fixed", FixedPrice: &fixedPrice}

	got := BuildInvoiceDraftSummary(project, nil, nil)
	want := "Projekt: Céges weboldal\nÖsszesen: 500 000 Ft (fix áras)"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildInvoiceDraftSummaryFixedPriceWithoutAmount(t *testing.T) {
	project := models.Project{Name: "Céges weboldal", PricingType: "fixed"}

	got := BuildInvoiceDraftSummary(project, nil, nil)
	if got != "" {
		t.Fatalf("expected empty summary, got %q", got)
	}
}

func TestBuildInvoiceDraftSummaryHourlyWithoutPeriod(t *testing.T) {
	rate := 20000.0
	project := models.Project{Name: "Website Redesign", PricingType: "hourly", HourlyRate: &rate}

	got := BuildInvoiceDraftSummary(project, nil, nil)
	if got != "" {
		t.Fatalf("expected empty summary without a period, got %q", got)
	}
}
