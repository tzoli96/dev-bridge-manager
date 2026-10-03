// backend/internal/services/unbilled_hours_test.go
package services

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func rate(v float64) *float64 { return &v }
func cid(v uint) *uint        { return &v }

// "Now" is 2026-10-15, so the cutoff is 2026-10-01: anything before it is
// "last month or older".
var ubNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func hourlyProject(id uint, name string, r *float64, auto *uint, clients ...uint) UnbilledProject {
	return UnbilledProject{ID: id, Name: name, PricingType: "hourly", HourlyRate: r, AutoInvoiceClientID: auto, ClientIDs: clients}
}

func ubInput(projects []UnbilledProject, entries []UnbilledEntry) UnbilledInput {
	return UnbilledInput{
		Now: ubNow, Projects: projects, Entries: entries,
		Periods:     map[uint][]InvoicedPeriod{},
		ClientNames: map[uint]string{1: "PIXEL", 2: "ACME"},
	}
}

func TestUnbilledCutoffIsFirstOfCurrentMonthUTC(t *testing.T) {
	got := unbilledCutoff(time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC))
	if !got.Equal(d(2026, 10, 1)) {
		t.Fatalf("cutoff = %v", got)
	}
	if got := unbilledCutoff(d(2026, 1, 1)); !got.Equal(d(2026, 1, 1)) {
		t.Fatalf("cutoff on the 1st = %v", got)
	}
}

func TestBuildUnbilledHoursCategorisesByDoneAndCutoff(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "Mentorfy", rate(10000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 30), Hours: 2, TaskDone: false}, // last month, open task -> billable
			{ProjectID: 7, Date: d(2026, 10, 1), Hours: 3, TaskDone: false}, // cutoff day, open -> in progress
			{ProjectID: 7, Date: d(2026, 10, 5), Hours: 4, TaskDone: true},  // this month, done -> billable
			{ProjectID: 7, Date: d(2026, 10, 9), Hours: 1, TaskDone: false}, // this month, open -> in progress
		},
	))
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 1 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	row := out.Clients[0].Projects[0]
	if !approx(row.BillableHours, 6) || !approx(row.InProgressHours, 4) || !approx(row.LoggedHours, 10) {
		t.Fatalf("row = %+v", row)
	}
	if row.BillableAmount == nil || !approx(*row.BillableAmount, 60000) {
		t.Fatalf("amount = %v", row.BillableAmount)
	}
	if !approx(out.Totals.BillableHours, 6) || !approx(out.Totals.BillableAmount, 60000) || !approx(out.Totals.InProgressHours, 4) {
		t.Fatalf("totals = %+v", out.Totals)
	}
}

func TestBuildUnbilledHoursInvoicedDatesAreExcluded(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(10000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 5, TaskDone: true},  // inside the invoiced period
			{ProjectID: 7, Date: d(2026, 9, 15), Hours: 2, TaskDone: true},  // period end is inclusive
			{ProjectID: 7, Date: d(2026, 9, 16), Hours: 3, TaskDone: false}, // after the period, last month -> billable
		},
	)
	in.Periods[7] = []InvoicedPeriod{{Start: d(2026, 9, 1), End: d(2026, 9, 15)}}
	row := BuildUnbilledHours(in).Clients[0].Projects[0]
	if !approx(row.BillableHours, 3) || !approx(row.LoggedHours, 10) {
		t.Fatalf("row = %+v", row)
	}
}

func TestBuildUnbilledHoursFullyInvoicedHourlyProjectIsOmittedButCounted(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{
			hourlyProject(7, "Paid", rate(10000), nil, 1),
			hourlyProject(8, "Open", rate(10000), nil, 1),
		},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 5, TaskDone: true},
			{ProjectID: 8, Date: d(2026, 9, 10), Hours: 1, TaskDone: true},
		},
	)
	in.Periods[7] = []InvoicedPeriod{{Start: d(2026, 9, 1), End: d(2026, 9, 30)}}
	out := BuildUnbilledHours(in)
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 1 || out.Clients[0].Projects[0].ProjectID != 8 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	if out.FullyInvoicedProjects != 1 {
		t.Fatalf("fully invoiced = %d", out.FullyInvoicedProjects)
	}
}

func TestBuildUnbilledHoursMissingRateGivesNilAmountAndWarning(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "NoRate", nil, nil, 1)},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 9, 10), Hours: 4, TaskDone: true}},
	))
	row := out.Clients[0].Projects[0]
	if row.BillableAmount != nil || !row.MissingRate || !approx(row.BillableHours, 4) {
		t.Fatalf("row = %+v", row)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
	if !approx(out.Totals.BillableHours, 4) || !approx(out.Totals.BillableAmount, 0) {
		t.Fatalf("totals = %+v", out.Totals)
	}
}

func TestBuildUnbilledHoursFixedAndHobbyAreListedWithoutBillableValues(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{
			{ID: 7, Name: "Fix", PricingType: "fixed", ClientIDs: []uint{1}},
			{ID: 8, Name: "Hobbi", PricingType: "hobby", ClientIDs: []uint{1}},
		},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 6, TaskDone: true},
			{ProjectID: 8, Date: d(2026, 10, 3), Hours: 2, TaskDone: false},
		},
	)
	out := BuildUnbilledHours(in)
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 2 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	for _, r := range out.Clients[0].Projects {
		if r.HourlyBased || r.BillableAmount != nil || !approx(r.BillableHours, 0) || !approx(r.InProgressHours, 0) {
			t.Fatalf("non-hourly row must carry no billable values: %+v", r)
		}
	}
	if !approx(out.Clients[0].Projects[0].LoggedHours+out.Clients[0].Projects[1].LoggedHours, 8) {
		t.Fatalf("logged hours lost: %+v", out.Clients[0].Projects)
	}
	if !approx(out.Totals.BillableHours, 0) || !approx(out.Totals.InProgressHours, 0) {
		t.Fatalf("totals must ignore non-hourly projects: %+v", out.Totals)
	}
}

func TestBuildUnbilledHoursClientResolution(t *testing.T) {
	entry := func(p uint) UnbilledEntry {
		return UnbilledEntry{ProjectID: p, Date: d(2026, 9, 10), Hours: 1, TaskDone: true}
	}
	in := ubInput(
		[]UnbilledProject{
			hourlyProject(1, "AutoSet", rate(1000), cid(2), 1, 2), // auto client wins over the two linked ones
			hourlyProject(2, "Sole", rate(1000), nil, 1),          // single linked client
			hourlyProject(3, "Many", rate(1000), nil, 1, 2),       // several, none designated
			hourlyProject(4, "None", rate(1000), nil),             // no client at all
		},
		[]UnbilledEntry{entry(1), entry(2), entry(3), entry(4)},
	)
	out := BuildUnbilledHours(in)

	byName := map[string][]string{}
	var unassigned *UnbilledClientGroup
	for i := range out.Clients {
		g := out.Clients[i]
		names := []string{}
		for _, p := range g.Projects {
			names = append(names, p.ProjectName)
		}
		if g.Unassigned {
			unassigned = &out.Clients[i]
		}
		byName[g.ClientName] = names
	}
	if len(byName["ACME"]) != 1 || byName["ACME"][0] != "AutoSet" {
		t.Fatalf("ACME group = %v", byName["ACME"])
	}
	if len(byName["PIXEL"]) != 1 || byName["PIXEL"][0] != "Sole" {
		t.Fatalf("PIXEL group = %v", byName["PIXEL"])
	}
	if unassigned == nil || len(unassigned.Projects) != 2 {
		t.Fatalf("unassigned = %+v", unassigned)
	}
	if out.Clients[len(out.Clients)-1].Unassigned != true {
		t.Fatalf("unassigned group must be last: %+v", out.Clients)
	}
	if len(out.Warnings) != 2 {
		t.Fatalf("expected one warning per unassigned project, got %v", out.Warnings)
	}
}

func TestBuildUnbilledHoursEachProjectAppearsOnceSoTotalsAreNotDoubled(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{hourlyProject(1, "Shared", rate(1000), cid(1), 1, 2)},
		[]UnbilledEntry{{ProjectID: 1, Date: d(2026, 9, 10), Hours: 3, TaskDone: true}},
	)
	out := BuildUnbilledHours(in)
	if !approx(out.Totals.BillableHours, 3) || !approx(out.Totals.BillableAmount, 3000) {
		t.Fatalf("totals = %+v", out.Totals)
	}
	count := 0
	for _, g := range out.Clients {
		count += len(g.Projects)
	}
	if count != 1 {
		t.Fatalf("project listed %d times", count)
	}
}

func TestBuildUnbilledHoursOldestBillableDate(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 20), Hours: 1, TaskDone: false},
			{ProjectID: 7, Date: d(2026, 8, 3), Hours: 1, TaskDone: false},
			{ProjectID: 7, Date: d(2026, 10, 2), Hours: 1, TaskDone: false}, // in progress, must not count
		},
	))
	row := out.Clients[0].Projects[0]
	if row.OldestBillableDate == nil || *row.OldestBillableDate != "2026-08-03" {
		t.Fatalf("oldest = %v", row.OldestBillableDate)
	}
	none := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), nil, 1)},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 10, 2), Hours: 1, TaskDone: false}},
	))
	if none.Clients[0].Projects[0].OldestBillableDate != nil {
		t.Fatalf("only in-progress hours must give no oldest date")
	}
}

func TestBuildUnbilledHoursSortsGroupsAndProjectsByAmountThenName(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{
			hourlyProject(1, "Small", rate(1000), nil, 1),
			hourlyProject(2, "Big", rate(1000), nil, 2),
			hourlyProject(3, "Bigger", rate(1000), nil, 2),
		},
		[]UnbilledEntry{
			{ProjectID: 1, Date: d(2026, 9, 1), Hours: 1, TaskDone: true},
			{ProjectID: 2, Date: d(2026, 9, 1), Hours: 5, TaskDone: true},
			{ProjectID: 3, Date: d(2026, 9, 1), Hours: 9, TaskDone: true},
		},
	)
	out := BuildUnbilledHours(in)
	if out.Clients[0].ClientName != "ACME" || out.Clients[1].ClientName != "PIXEL" {
		t.Fatalf("group order = %s, %s", out.Clients[0].ClientName, out.Clients[1].ClientName)
	}
	if out.Clients[0].Projects[0].ProjectName != "Bigger" || out.Clients[0].Projects[1].ProjectName != "Big" {
		t.Fatalf("project order = %+v", out.Clients[0].Projects)
	}
}

func TestBuildUnbilledHoursUnknownClientNameFallsBackToID(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), cid(99))},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 9, 10), Hours: 1, TaskDone: true}},
	))
	if out.Clients[0].ClientName != "#99" || out.Clients[0].Unassigned {
		t.Fatalf("group = %+v", out.Clients[0])
	}
}

func TestBuildUnbilledHoursIgnoresEntriesOfUnknownProjectsAndZeroHours(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 99, Date: d(2026, 9, 10), Hours: 5, TaskDone: true}, // no such project
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 0, TaskDone: true},  // zero hours
		},
	))
	if len(out.Clients) != 0 {
		t.Fatalf("groups = %+v", out.Clients)
	}
}

func TestBuildUnbilledHoursEmptyInputHasNonNilSlices(t *testing.T) {
	out := BuildUnbilledHours(UnbilledInput{Now: ubNow})
	if out.Clients == nil || out.Warnings == nil {
		t.Fatalf("nil slice in %+v", out)
	}
	if out.CutoffDate != "2026-10-01" || out.AsOf != "2026-10-15" {
		t.Fatalf("dates = %s / %s", out.CutoffDate, out.AsOf)
	}
}

func TestBuildUnbilledHoursClientGroupTotals(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{
			hourlyProject(1, "A", rate(1000), nil, 1),
			hourlyProject(2, "B", rate(2000), nil, 1),
		},
		[]UnbilledEntry{
			{ProjectID: 1, Date: d(2026, 9, 1), Hours: 2, TaskDone: true},
			{ProjectID: 2, Date: d(2026, 9, 1), Hours: 3, TaskDone: true},
			{ProjectID: 2, Date: d(2026, 10, 4), Hours: 1, TaskDone: false},
		},
	))
	g := out.Clients[0]
	if !approx(g.BillableHours, 5) || !approx(g.BillableAmount, 8000) || !approx(g.InProgressHours, 1) {
		t.Fatalf("group = %+v", g)
	}
}

func TestUnbilledCutoffUsesUTCForNonUTCNow(t *testing.T) {
	// 2026-10-01 00:30 CEST == 2026-09-30 22:30 UTC
	now := time.Date(2026, 10, 1, 0, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	if got := unbilledCutoff(now); !got.Equal(d(2026, 9, 1)) {
		t.Fatalf("cutoff = %v", got)
	}
	out := BuildUnbilledHours(UnbilledInput{Now: now})
	if out.CutoffDate != "2026-09-01" || out.AsOf != "2026-09-30" {
		t.Fatalf("dates = %s / %s", out.CutoffDate, out.AsOf)
	}
}

func TestBuildUnbilledHoursTiedGroupsOrderByClientID(t *testing.T) {
	for i := 0; i < 20; i++ {
		in := ubInput(
			[]UnbilledProject{
				hourlyProject(1, "P1", rate(1000), nil, 1),
				hourlyProject(2, "P2", rate(1000), nil, 2),
			},
			[]UnbilledEntry{
				{ProjectID: 1, Date: d(2026, 9, 1), Hours: 2, TaskDone: true},
				{ProjectID: 2, Date: d(2026, 9, 1), Hours: 2, TaskDone: true},
			},
		)
		in.ClientNames = map[uint]string{1: "X", 2: "X"}
		out := BuildUnbilledHours(in)
		if len(out.Clients) != 2 || *out.Clients[0].ClientID != 1 || *out.Clients[1].ClientID != 2 {
			t.Fatalf("iteration %d: order = %+v", i, out.Clients)
		}
	}
}

func TestBuildUnbilledHoursIgnoresNegativeHours(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{
			hourlyProject(7, "P", rate(1000), nil, 1),
			hourlyProject(8, "OnlyNeg", rate(1000), nil, 1),
		},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 3, TaskDone: true},
			{ProjectID: 7, Date: d(2026, 9, 11), Hours: -2, TaskDone: true},
			{ProjectID: 8, Date: d(2026, 9, 10), Hours: -2, TaskDone: true},
		},
	))
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 1 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	row := out.Clients[0].Projects[0]
	if row.ProjectID != 7 || !approx(row.LoggedHours, 3) || !approx(row.BillableHours, 3) {
		t.Fatalf("row = %+v", row)
	}
	if row.BillableAmount == nil || !approx(*row.BillableAmount, 3000) {
		t.Fatalf("amount = %v", row.BillableAmount)
	}
}

func TestBuildUnbilledHoursNonHourlyDoesNotCountAsFullyInvoiced(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{{ID: 7, Name: "Fix", PricingType: "fixed", ClientIDs: []uint{1}}},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 9, 10), Hours: 6, TaskDone: true}},
	))
	if out.FullyInvoicedProjects != 0 {
		t.Fatalf("fully invoiced = %d", out.FullyInvoicedProjects)
	}
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 1 {
		t.Fatalf("groups = %+v", out.Clients)
	}
}
