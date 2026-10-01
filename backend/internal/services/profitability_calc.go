// backend/internal/services/profitability_calc.go
package services

import (
	"fmt"
	"sort"
	"time"

	"dev-bridge-manager/internal/models"
)

// lowDataMonths is the minimum number of months with activity before a row's
// rates are presented without a "low data" warning.
const lowDataMonths = 3

type InvoiceMonth struct {
	ProjectID uint
	ClientID  uint
	Month     string // "YYYY-MM"
	Amount    float64
}

type ProjectHoursMonth struct {
	ProjectID uint
	Month     string
	Hours     float64
}

type EmailMonth struct {
	ClientID  uint
	ProjectID *uint
	Month     string
	Folder    string // "inbox" | "sent"
	Count     int
}

type ProjectClientLink struct {
	ProjectID uint
	ClientID  uint
}

type OverviewInput struct {
	Months       []string
	Invoices     []InvoiceMonth
	ProjectHours []ProjectHoursMonth
	Emails       []EmailMonth
	Links        []ProjectClientLink
	ClientNames  map[uint]string
	ProjectNames map[uint]string
	MeetingHours map[uint]float64 // per client, hours per month
	Settings     models.ProfitSettings
}

// RateRow is one client or project line of the overview. Rates are pointers
// so "no hours logged" serialises as null instead of a misleading 0 or Inf.
type RateRow struct {
	ID                   uint     `json:"id"`
	Name                 string   `json:"name"`
	Revenue              float64  `json:"revenue"`
	LoggedHours          float64  `json:"logged_hours"`
	EmailHours           float64  `json:"email_hours"`
	MeetingHours         float64  `json:"meeting_hours"`
	MeetingHoursPerMonth float64  `json:"meeting_hours_per_month"`
	NominalRate          *float64 `json:"nominal_rate"`
	RealRate             *float64 `json:"real_rate"`
	Ratio                *float64 `json:"ratio"`
	UnderpricedCandidate bool     `json:"underpriced_candidate"`
	MonthsWithData       int      `json:"months_with_data"`
	LowData              bool     `json:"low_data"`
}

type Overview struct {
	Months   []string              `json:"months"`
	Clients  []RateRow             `json:"clients"`
	Projects []RateRow             `json:"projects"`
	Warnings []string              `json:"warnings"`
	Settings models.ProfitSettings `json:"settings"`
}

// MonthWindow returns the n complete calendar months before now's month, as
// "YYYY-MM" labels, plus the [from, to) instants covering them.
func MonthWindow(now time.Time, n int) (months []string, from, to time.Time) {
	to = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	from = to.AddDate(0, -n, 0)
	for i := 0; i < n; i++ {
		months = append(months, from.AddDate(0, i, 0).Format("2006-01"))
	}
	return months, from, to
}

// AllocateHours splits hours across ids proportionally to weights; when all
// weights are zero it splits equally across fallbackIDs; with neither it
// returns an empty map (the caller reports the hours as unattributed).
func AllocateHours(hours float64, weights map[uint]float64, fallbackIDs []uint) map[uint]float64 {
	out := map[uint]float64{}
	var total float64
	for _, w := range weights {
		total += w
	}
	if total > 0 {
		for id, w := range weights {
			out[id] = hours * w / total
		}
		return out
	}
	if len(fallbackIDs) == 0 {
		return out
	}
	share := hours / float64(len(fallbackIDs))
	for _, id := range fallbackIDs {
		out[id] = share
	}
	return out
}

func BuildOverview(in OverviewInput) Overview {
	clients := map[uint]*RateRow{}
	projects := map[uint]*RateRow{}
	clientMonths := map[uint]map[string]bool{}
	projectMonths := map[uint]map[string]bool{}

	rowFor := func(rows map[uint]*RateRow, names map[uint]string, id uint) *RateRow {
		r, ok := rows[id]
		if !ok {
			name := names[id]
			if name == "" {
				name = fmt.Sprintf("#%d", id)
			}
			r = &RateRow{ID: id, Name: name}
			rows[id] = r
		}
		return r
	}
	mark := func(m map[uint]map[string]bool, id uint, month string) {
		if m[id] == nil {
			m[id] = map[string]bool{}
		}
		m[id][month] = true
	}

	linksByProject := map[uint][]uint{}
	for _, l := range in.Links {
		linksByProject[l.ProjectID] = append(linksByProject[l.ProjectID], l.ClientID)
	}

	// project -> month -> client -> invoiced amount, used as allocation weights.
	weights := map[uint]map[string]map[uint]float64{}
	for _, inv := range in.Invoices {
		rowFor(clients, in.ClientNames, inv.ClientID).Revenue += inv.Amount
		rowFor(projects, in.ProjectNames, inv.ProjectID).Revenue += inv.Amount
		mark(clientMonths, inv.ClientID, inv.Month)
		mark(projectMonths, inv.ProjectID, inv.Month)
		if weights[inv.ProjectID] == nil {
			weights[inv.ProjectID] = map[string]map[uint]float64{}
		}
		if weights[inv.ProjectID][inv.Month] == nil {
			weights[inv.ProjectID][inv.Month] = map[uint]float64{}
		}
		weights[inv.ProjectID][inv.Month][inv.ClientID] += inv.Amount
	}

	var unattributed float64
	for _, ph := range in.ProjectHours {
		rowFor(projects, in.ProjectNames, ph.ProjectID).LoggedHours += ph.Hours
		mark(projectMonths, ph.ProjectID, ph.Month)
		alloc := AllocateHours(ph.Hours, weights[ph.ProjectID][ph.Month], linksByProject[ph.ProjectID])
		for clientID, h := range alloc {
			rowFor(clients, in.ClientNames, clientID).LoggedHours += h
			mark(clientMonths, clientID, ph.Month)
		}
		if len(alloc) == 0 {
			unattributed += ph.Hours
		}
	}

	for _, e := range in.Emails {
		minutes := in.Settings.MinutesPerInboundEmail
		if e.Folder == "sent" {
			minutes = in.Settings.MinutesPerOutboundEmail
		}
		h := float64(e.Count) * minutes / 60
		rowFor(clients, in.ClientNames, e.ClientID).EmailHours += h
		if e.ProjectID != nil {
			rowFor(projects, in.ProjectNames, *e.ProjectID).EmailHours += h
		}
	}

	finalize := func(rows map[uint]*RateRow, months map[uint]map[string]bool, withMeetings bool) []RateRow {
		out := make([]RateRow, 0, len(rows))
		for id, r := range rows {
			if withMeetings {
				r.MeetingHoursPerMonth = in.MeetingHours[id]
				r.MeetingHours = in.MeetingHours[id] * float64(len(in.Months))
			}
			if r.LoggedHours > 0 {
				v := r.Revenue / r.LoggedHours
				r.NominalRate = &v
			}
			if total := r.LoggedHours + r.EmailHours + r.MeetingHours; total > 0 {
				v := r.Revenue / total
				r.RealRate = &v
			}
			if r.NominalRate != nil && r.RealRate != nil && *r.NominalRate > 0 {
				v := *r.RealRate / *r.NominalRate
				r.Ratio = &v
				r.UnderpricedCandidate = v < in.Settings.UnderpricedRatioThreshold
			}
			r.MonthsWithData = len(months[id])
			r.LowData = r.MonthsWithData < lowDataMonths
			out = append(out, *r)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Revenue != out[j].Revenue {
				return out[i].Revenue > out[j].Revenue
			}
			return out[i].ID < out[j].ID
		})
		return out
	}

	warnings := []string{}
	if unattributed > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%.1f naplózott óra nem rendelhető ügyfélhez (nincs számla vagy projekt–ügyfél kapcsolat)", unattributed))
	}

	return Overview{
		Months:   in.Months,
		Clients:  finalize(clients, clientMonths, true),
		Projects: finalize(projects, projectMonths, false),
		Warnings: warnings,
		Settings: in.Settings,
	}
}
