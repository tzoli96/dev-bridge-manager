// backend/internal/services/profitability_calc_test.go
package services

import (
	"math"
	"testing"
	"time"

	"dev-bridge-manager/internal/models"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestMonthWindow(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	months, from, to := MonthWindow(now, 3)
	want := []string{"2026-07", "2026-08", "2026-09"}
	for i, m := range want {
		if months[i] != m {
			t.Fatalf("months[%d] = %s, want %s", i, months[i], m)
		}
	}
	if !from.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %v", from)
	}
	if !to.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to = %v", to)
	}
}

func TestAllocateHoursProportionalToWeights(t *testing.T) {
	got := AllocateHours(10, map[uint]float64{1: 300, 2: 100}, []uint{1, 2})
	if !approx(got[1], 7.5) || !approx(got[2], 2.5) {
		t.Fatalf("got %v", got)
	}
}

func TestAllocateHoursFallsBackToEqualSplit(t *testing.T) {
	got := AllocateHours(10, nil, []uint{1, 2})
	if !approx(got[1], 5) || !approx(got[2], 5) {
		t.Fatalf("got %v", got)
	}
}

func TestAllocateHoursNothingToAllocateTo(t *testing.T) {
	if got := AllocateHours(10, nil, nil); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func baseSettings() models.ProfitSettings {
	return models.ProfitSettings{
		MinutesPerInboundEmail:    5,
		MinutesPerOutboundEmail:   10,
		UnderpricedRatioThreshold: 0.9,
	}
}

func TestBuildOverviewRealRateIncludesEmailAndMeetingTime(t *testing.T) {
	pid := uint(7)
	in := OverviewInput{
		Months:       []string{"2026-08", "2026-09"},
		Invoices:     []InvoiceMonth{{ProjectID: 7, ClientID: 1, Month: "2026-08", Amount: 100000}, {ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 100000}},
		ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-08", Hours: 10}, {ProjectID: 7, Month: "2026-09", Hours: 10}},
		Emails: []EmailMonth{
			{ClientID: 1, ProjectID: &pid, Month: "2026-08", Folder: "inbox", Count: 12}, // 12*5min = 1h
			{ClientID: 1, ProjectID: &pid, Month: "2026-08", Folder: "sent", Count: 6},   // 6*10min = 1h
		},
		Links:        []ProjectClientLink{{ProjectID: 7, ClientID: 1}},
		ClientNames:  map[uint]string{1: "PIXEL"},
		ProjectNames: map[uint]string{7: "Mentorfy"},
		MeetingHours: map[uint]float64{1: 1}, // 1h/month * 2 months
		Settings:     baseSettings(),
	}

	out := BuildOverview(in)
	if len(out.Clients) != 1 {
		t.Fatalf("clients = %d", len(out.Clients))
	}
	c := out.Clients[0]
	if !approx(c.Revenue, 200000) || !approx(c.LoggedHours, 20) || !approx(c.EmailHours, 2) || !approx(c.MeetingHours, 2) {
		t.Fatalf("client row = %+v", c)
	}
	if c.NominalRate == nil || !approx(*c.NominalRate, 10000) {
		t.Fatalf("nominal = %v", c.NominalRate)
	}
	if c.RealRate == nil || !approx(*c.RealRate, 200000.0/24.0) {
		t.Fatalf("real = %v", c.RealRate)
	}
	if c.Ratio == nil || !approx(*c.Ratio, (200000.0/24.0)/10000.0) {
		t.Fatalf("ratio = %v", c.Ratio)
	}
	if !c.UnderpricedCandidate {
		t.Fatalf("ratio %.3f under threshold 0.9 should flag", *c.Ratio)
	}
	if !c.LowData {
		t.Fatalf("2 months of data should be flagged low-data")
	}

	// Project rows carry email time but no meeting flat-rate (it is per client).
	p := out.Projects[0]
	if !approx(p.EmailHours, 2) || !approx(p.MeetingHours, 0) {
		t.Fatalf("project row = %+v", p)
	}
}

func TestBuildOverviewNoHoursMeansNoRates(t *testing.T) {
	in := OverviewInput{
		Months:      []string{"2026-09"},
		Invoices:    []InvoiceMonth{{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 50000}},
		ClientNames: map[uint]string{1: "A"}, ProjectNames: map[uint]string{7: "P"},
		Settings: baseSettings(),
	}
	c := BuildOverview(in).Clients[0]
	if c.NominalRate != nil || c.RealRate != nil || c.Ratio != nil || c.UnderpricedCandidate {
		t.Fatalf("expected no rates without hours, got %+v", c)
	}
}

func TestBuildOverviewAllocatesHoursByInvoicedShare(t *testing.T) {
	in := OverviewInput{
		Months: []string{"2026-09"},
		Invoices: []InvoiceMonth{
			{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 300},
			{ProjectID: 7, ClientID: 2, Month: "2026-09", Amount: 100},
		},
		ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-09", Hours: 8}},
		Links:        []ProjectClientLink{{ProjectID: 7, ClientID: 1}, {ProjectID: 7, ClientID: 2}},
		ClientNames:  map[uint]string{1: "A", 2: "B"}, ProjectNames: map[uint]string{7: "P"},
		Settings: baseSettings(),
	}
	out := BuildOverview(in)
	hours := map[uint]float64{}
	for _, c := range out.Clients {
		hours[c.ID] = c.LoggedHours
	}
	if !approx(hours[1], 6) || !approx(hours[2], 2) {
		t.Fatalf("hours = %v", hours)
	}
}

func TestBuildOverviewWarnsAboutUnattributedHours(t *testing.T) {
	in := OverviewInput{
		Months:       []string{"2026-09"},
		ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-09", Hours: 4}},
		ProjectNames: map[uint]string{7: "P"},
		Settings:     baseSettings(),
	}
	out := BuildOverview(in)
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestBuildOverviewSkipsClientsWithoutActivity(t *testing.T) {
	in := OverviewInput{
		Months:       []string{"2026-09"},
		ClientNames:  map[uint]string{1: "Idle"},
		MeetingHours: map[uint]float64{1: 2},
		Settings:     baseSettings(),
	}
	if out := BuildOverview(in); len(out.Clients) != 0 {
		t.Fatalf("idle client should not appear, got %+v", out.Clients)
	}
}

func TestBuildOverviewNoFalseUnattributedWarningFromRounding(t *testing.T) {
	for n := 2; n <= 12; n++ {
		for _, hours := range []float64{0.25, 0.1, 1.0 / 3.0, 7.7} {
			var links []ProjectClientLink
			names := map[uint]string{}
			for i := 1; i <= n; i++ {
				links = append(links, ProjectClientLink{ProjectID: 7, ClientID: uint(i)})
				names[uint(i)] = "C"
			}
			in := OverviewInput{
				Months:       []string{"2026-09"},
				ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-09", Hours: hours}},
				Links:        links,
				ClientNames:  names,
				ProjectNames: map[uint]string{7: "P"},
				Settings:     baseSettings(),
			}
			for rep := 0; rep < 20; rep++ {
				if out := BuildOverview(in); len(out.Warnings) != 0 {
					t.Fatalf("n=%d hours=%v: false warning %v", n, hours, out.Warnings)
				}
			}
		}
	}
}
