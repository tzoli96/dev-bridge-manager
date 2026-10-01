// backend/internal/handlers/profitability_handler_test.go
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestParseOverviewMonths(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"", 3, true},
		{"6", 6, true},
		{"24", 24, true},
		{"0", 0, false},
		{"25", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseOverviewMonths(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("parseOverviewMonths(%q) = %d,%v want %d,%v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestValidateProfitSettings(t *testing.T) {
	good := models.ProfitSettingsRequest{
		MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10,
		DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.6,
	}
	if msg := validateProfitSettings(good); msg != "" {
		t.Fatalf("good settings rejected: %s", msg)
	}
	min := good
	min.UnderpricedRatioThreshold = 0.01
	if msg := validateProfitSettings(min); msg != "" {
		t.Fatalf("threshold 0.01 rejected: %s", msg)
	}
	bad := []models.ProfitSettingsRequest{
		{MinutesPerInboundEmail: -1, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 241, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 745, UnderpricedRatioThreshold: 0.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 1.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.004},
	}
	for i, b := range bad {
		if msg := validateProfitSettings(b); msg == "" {
			t.Fatalf("bad settings #%d accepted: %+v", i, b)
		}
	}
}

func TestValidateMeetingAllowance(t *testing.T) {
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: 0}); msg != "" {
		t.Fatalf("zero rejected: %s", msg)
	}
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: 744}); msg != "" {
		t.Fatalf("744 rejected: %s", msg)
	}
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: -1}); msg == "" {
		t.Fatal("negative accepted")
	}
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: 745}); msg == "" {
		t.Fatal("745 accepted")
	}
}

func TestParseForecastMonths(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"", 6, true},
		{"3", 3, true},
		{"6", 6, true},
		{"2", 0, false},
		{"7", 0, false},
		{"0", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseForecastMonths(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("parseForecastMonths(%q) = %d,%v want %d,%v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
