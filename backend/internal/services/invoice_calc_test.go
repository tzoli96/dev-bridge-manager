// backend/internal/services/invoice_calc_test.go
package services

import "testing"

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
