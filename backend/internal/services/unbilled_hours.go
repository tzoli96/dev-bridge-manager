// backend/internal/services/unbilled_hours.go
package services

import (
	"fmt"
	"sort"
	"time"
)

// UnbilledEntry is one logged time entry. TaskDone is true when the entry's
// task currently sits in a kanban column flagged is_done.
type UnbilledEntry struct {
	ProjectID uint
	Date      time.Time
	Hours     float64
	TaskDone  bool
}

type UnbilledProject struct {
	ID                  uint
	Name                string
	PricingType         string
	HourlyRate          *float64
	AutoInvoiceClientID *uint
	ClientIDs           []uint
}

type UnbilledInput struct {
	Now         time.Time
	Projects    []UnbilledProject
	Entries     []UnbilledEntry
	Periods     map[uint][]InvoicedPeriod // already-invoiced hourly periods per project
	ClientNames map[uint]string
}

type UnbilledProjectRow struct {
	ProjectID          uint     `json:"project_id"`
	ProjectName        string   `json:"project_name"`
	PricingType        string   `json:"pricing_type"`
	HourlyBased        bool     `json:"hourly_based"`
	HourlyRate         *float64 `json:"hourly_rate"`
	LoggedHours        float64  `json:"logged_hours"`
	BillableHours      float64  `json:"billable_hours"`
	BillableAmount     *float64 `json:"billable_amount"`
	InProgressHours    float64  `json:"in_progress_hours"`
	OldestBillableDate *string  `json:"oldest_billable_date"`
	MissingRate        bool     `json:"missing_rate"`
}

type UnbilledClientGroup struct {
	ClientID        *uint                `json:"client_id"`
	ClientName      string               `json:"client_name"`
	Unassigned      bool                 `json:"unassigned"`
	BillableHours   float64              `json:"billable_hours"`
	BillableAmount  float64              `json:"billable_amount"`
	InProgressHours float64              `json:"in_progress_hours"`
	Projects        []UnbilledProjectRow `json:"projects"`
}

type UnbilledTotals struct {
	BillableHours   float64 `json:"billable_hours"`
	BillableAmount  float64 `json:"billable_amount"`
	InProgressHours float64 `json:"in_progress_hours"`
}

type UnbilledHours struct {
	AsOf                  string                `json:"as_of"`
	CutoffDate            string                `json:"cutoff_date"`
	Totals                UnbilledTotals        `json:"totals"`
	Clients               []UnbilledClientGroup `json:"clients"`
	FullyInvoicedProjects int                   `json:"fully_invoiced_projects"`
	Warnings              []string              `json:"warnings"`
}

// unbilledCutoff returns the first day of now's calendar month (UTC date).
// Hours dated before it count as "last month or older".
func unbilledCutoff(now time.Time) time.Time {
	u := now.UTC()
	return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// BuildUnbilledHours splits every project's logged hours into invoiced,
// billable-now (unbilled AND (task done OR dated before the current month))
// and in-progress. "Invoiced" is IsDateInvoiced over the project's created
// hourly invoices, so it is the same rule the kanban card icon uses. Each
// project is placed under exactly one billing client so totals never double
// count. Only hourly projects carry billable values.
func BuildUnbilledHours(in UnbilledInput) UnbilledHours {
	cutoff := unbilledCutoff(in.Now)
	out := UnbilledHours{
		AsOf:       in.Now.UTC().Format("2006-01-02"),
		CutoffDate: cutoff.Format("2006-01-02"),
		Clients:    []UnbilledClientGroup{},
		Warnings:   []string{},
	}

	projects := make(map[uint]UnbilledProject, len(in.Projects))
	for _, p := range in.Projects {
		projects[p.ID] = p
	}

	type agg struct {
		logged, billable, inProgress float64
		oldest                       time.Time
		hasOldest                    bool
	}
	aggs := map[uint]*agg{}
	for _, e := range in.Entries {
		p, ok := projects[e.ProjectID]
		if !ok || e.Hours <= 0 {
			continue
		}
		a := aggs[e.ProjectID]
		if a == nil {
			a = &agg{}
			aggs[e.ProjectID] = a
		}
		a.logged += e.Hours
		if p.PricingType != "hourly" || IsDateInvoiced(e.Date, in.Periods[e.ProjectID]) {
			continue
		}
		if e.TaskDone || e.Date.Before(cutoff) {
			a.billable += e.Hours
			if !a.hasOldest || e.Date.Before(a.oldest) {
				a.oldest, a.hasOldest = e.Date, true
			}
		} else {
			a.inProgress += e.Hours
		}
	}

	ids := make([]uint, 0, len(aggs))
	for id := range aggs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] }) // deterministic float sums

	groups := map[uint]*UnbilledClientGroup{} // key 0 = unassigned
	for _, id := range ids {
		p, a := projects[id], aggs[id]
		hourly := p.PricingType == "hourly"

		if hourly && a.billable == 0 && a.inProgress == 0 {
			out.FullyInvoicedProjects++
			continue
		}

		row := UnbilledProjectRow{
			ProjectID: p.ID, ProjectName: p.Name, PricingType: p.PricingType,
			HourlyBased: hourly, HourlyRate: p.HourlyRate, LoggedHours: a.logged,
		}
		if hourly {
			row.BillableHours = a.billable
			row.InProgressHours = a.inProgress
			if a.hasOldest {
				s := a.oldest.Format("2006-01-02")
				row.OldestBillableDate = &s
			}
			if p.HourlyRate != nil {
				amount := a.billable * *p.HourlyRate
				row.BillableAmount = &amount
			} else if a.billable > 0 {
				row.MissingRate = true
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"A(z) %s projektnek nincs óradíja, ezért a számlázandó összeg nem számolható", p.Name))
			}
		}

		var key uint
		switch {
		case p.AutoInvoiceClientID != nil:
			key = *p.AutoInvoiceClientID
		case len(p.ClientIDs) == 1:
			key = p.ClientIDs[0]
		default:
			out.Warnings = append(out.Warnings, unassignedWarning(p))
		}

		g := groups[key]
		if g == nil {
			g = &UnbilledClientGroup{Projects: []UnbilledProjectRow{}}
			if key == 0 {
				g.Unassigned = true
				g.ClientName = "Nincs kijelölt számlázandó ügyfél"
			} else {
				k := key
				g.ClientID = &k
				g.ClientName = in.ClientNames[key]
				if g.ClientName == "" {
					g.ClientName = fmt.Sprintf("#%d", key)
				}
			}
			groups[key] = g
		}
		g.Projects = append(g.Projects, row)
		if hourly {
			g.BillableHours += row.BillableHours
			g.InProgressHours += row.InProgressHours
			if row.BillableAmount != nil {
				g.BillableAmount += *row.BillableAmount
			}
			out.Totals.BillableHours += row.BillableHours
			out.Totals.InProgressHours += row.InProgressHours
			if row.BillableAmount != nil {
				out.Totals.BillableAmount += *row.BillableAmount
			}
		}
	}

	for _, g := range groups {
		sort.SliceStable(g.Projects, func(i, j int) bool {
			ai, aj := amountOrZero(g.Projects[i]), amountOrZero(g.Projects[j])
			if ai != aj {
				return ai > aj
			}
			if g.Projects[i].BillableHours != g.Projects[j].BillableHours {
				return g.Projects[i].BillableHours > g.Projects[j].BillableHours
			}
			return g.Projects[i].ProjectName < g.Projects[j].ProjectName
		})
		out.Clients = append(out.Clients, *g)
	}
	sort.SliceStable(out.Clients, func(i, j int) bool {
		a, b := out.Clients[i], out.Clients[j]
		if a.Unassigned != b.Unassigned {
			return !a.Unassigned // the unassigned group goes last
		}
		if a.BillableAmount != b.BillableAmount {
			return a.BillableAmount > b.BillableAmount
		}
		if a.ClientName != b.ClientName {
			return a.ClientName < b.ClientName
		}
		if a.ClientID != nil && b.ClientID != nil {
			return *a.ClientID < *b.ClientID
		}
		return false
	})
	return out
}

func amountOrZero(r UnbilledProjectRow) float64 {
	if r.BillableAmount == nil {
		return 0
	}
	return *r.BillableAmount
}

func unassignedWarning(p UnbilledProject) string {
	if len(p.ClientIDs) == 0 {
		return fmt.Sprintf("A(z) %s projekthez nincs ügyfél rendelve, ezért nem derül ki, kinek kell számlázni", p.Name)
	}
	return fmt.Sprintf("A(z) %s projekt több ügyfélhez tartozik, de nincs kijelölt számlázandó ügyfél (automatikus számlázás ügyfele)", p.Name)
}
