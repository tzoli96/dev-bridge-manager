// backend/internal/services/unbilled_hours_load.go
package services

import (
	"time"

	"dev-bridge-manager/internal/database"

	"gorm.io/gorm"
)

// LoadUnbilledHours gathers every project's logged hours, the billed periods
// and the client links, and hands them to the pure BuildUnbilledHours.
func LoadUnbilledHours(now time.Time) (UnbilledHours, error) {
	db := database.GetDB()

	entries, err := loadUnbilledEntries(db)
	if err != nil {
		return UnbilledHours{}, err
	}
	projects, err := loadUnbilledProjects(db)
	if err != nil {
		return UnbilledHours{}, err
	}

	hourlyIDs := make([]uint, 0, len(projects))
	for _, p := range projects {
		if p.PricingType == "hourly" {
			hourlyIDs = append(hourlyIDs, p.ID)
		}
	}
	periods, err := InvoicedPeriodsByProject(hourlyIDs)
	if err != nil {
		return UnbilledHours{}, err
	}

	clientNames, err := loadNames(db, "clients")
	if err != nil {
		return UnbilledHours{}, err
	}

	return BuildUnbilledHours(UnbilledInput{
		Now: now, Projects: projects, Entries: entries, Periods: periods, ClientNames: clientNames,
	}), nil
}

// task_done is true when any of the task's placements is in an is_done
// column (the same notion tasksInDoneColumn uses for the kanban DTOs).
// Archived tasks are included on purpose: the invoice sums their hours too.
func loadUnbilledEntries(db *gorm.DB) ([]UnbilledEntry, error) {
	var rows []UnbilledEntry
	err := db.Table("task_time_entries").
		Select(`tasks.project_id AS project_id, task_time_entries.date AS date, task_time_entries.hours AS hours,
			EXISTS (SELECT 1 FROM task_placements tp JOIN kanban_columns kc ON kc.id = tp.column_id
				WHERE tp.task_id = tasks.id AND kc.is_done = true) AS task_done`).
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("task_time_entries.hours > 0").
		Scan(&rows).Error
	return rows, err
}

func loadUnbilledProjects(db *gorm.DB) ([]UnbilledProject, error) {
	var rows []struct {
		ID                  uint
		Name                string
		PricingType         string
		HourlyRate          *float64
		AutoInvoiceClientID *uint
	}
	if err := db.Table("projects").
		Select("id, name, COALESCE(pricing_type, '') AS pricing_type, hourly_rate, auto_invoice_client_id").
		Order("id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	var links []struct {
		ProjectID uint
		ClientID  uint
	}
	if err := db.Table("project_clients").Select("project_id, client_id").Order("client_id ASC").Scan(&links).Error; err != nil {
		return nil, err
	}
	clientsByProject := make(map[uint][]uint, len(rows))
	for _, l := range links {
		clientsByProject[l.ProjectID] = append(clientsByProject[l.ProjectID], l.ClientID)
	}

	projects := make([]UnbilledProject, 0, len(rows))
	for _, r := range rows {
		projects = append(projects, UnbilledProject{
			ID: r.ID, Name: r.Name, PricingType: r.PricingType, HourlyRate: r.HourlyRate,
			AutoInvoiceClientID: r.AutoInvoiceClientID, ClientIDs: clientsByProject[r.ID],
		})
	}
	return projects, nil
}
