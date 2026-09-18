// backend/internal/services/invoice_calc.go
package services

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"time"
)

// CalculateFixedAmount returns the full fixed price as the invoice amount.
func CalculateFixedAmount(fixedPrice float64) float64 {
	return fixedPrice
}

// CalculateHourlyAmount returns totalHours billed at hourlyRate.
func CalculateHourlyAmount(totalHours, hourlyRate float64) float64 {
	return totalHours * hourlyRate
}

// SumLoggedHours sums task_time_entries.hours for all entries whose task
// belongs to projectID and whose date falls within [periodStart, periodEnd].
func SumLoggedHours(projectID uint, periodStart, periodEnd time.Time) (float64, error) {
	var total float64
	err := database.GetDB().Table("task_time_entries").
		Select("COALESCE(SUM(task_time_entries.hours), 0)").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("tasks.project_id = ? AND task_time_entries.date BETWEEN ? AND ?", projectID, periodStart, periodEnd).
		Scan(&total).Error
	return total, err
}

// HourlyLineItem is one billed time entry within a period, joined with its
// task title and the user who logged it (used to show an invoice's
// hours/tasks breakdown).
type HourlyLineItem struct {
	TaskID    uint
	TaskTitle string
	BoardID   uint
	Date      time.Time
	Hours     float64
	UserName  string
}

// GetHourlyLineItems returns every time entry that SumLoggedHours would sum
// for the same project/period, joined with task and user info, so callers
// can show exactly which hours/tasks a given hourly invoice covered.
func GetHourlyLineItems(projectID uint, periodStart, periodEnd time.Time) ([]HourlyLineItem, error) {
	var items []HourlyLineItem
	// A task can be placed on more than one board (task_placements has a
	// task_id/board_id unique pair, not a unique task_id), so board_id is
	// resolved via a correlated subquery picking one placement rather than
	// a JOIN, which would otherwise duplicate/inflate the billed hours.
	err := database.GetDB().Table("task_time_entries").
		Select("tasks.id AS task_id, tasks.title AS task_title, COALESCE((SELECT tp.board_id FROM task_placements tp WHERE tp.task_id = tasks.id ORDER BY tp.id LIMIT 1), 0) AS board_id, task_time_entries.date AS date, task_time_entries.hours AS hours, COALESCE(users.name, '') AS user_name").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Joins("LEFT JOIN users ON users.id = task_time_entries.user_id").
		Where("tasks.project_id = ? AND task_time_entries.date BETWEEN ? AND ?", projectID, periodStart, periodEnd).
		Order("task_time_entries.date ASC").
		Scan(&items).Error
	return items, err
}

// InvoicedPeriod is the billed date range of one successfully created hourly
// invoice.
type InvoicedPeriod struct {
	Start time.Time
	End   time.Time
}

// InvoicedPeriodsByProject returns the billed periods of every successfully
// created hourly invoice for the given projects, so callers (e.g. the kanban
// board) can flag which already-logged hours have been invoiced without
// tracking individual entries.
func InvoicedPeriodsByProject(projectIDs []uint) (map[uint][]InvoicedPeriod, error) {
	result := make(map[uint][]InvoicedPeriod)
	if len(projectIDs) == 0 {
		return result, nil
	}

	var invoices []models.Invoice
	err := database.GetDB().
		Where("project_id IN ? AND status = 'created' AND pricing_type = 'hourly' AND period_start IS NOT NULL AND period_end IS NOT NULL", projectIDs).
		Find(&invoices).Error
	if err != nil {
		return nil, err
	}

	for _, inv := range invoices {
		result[inv.ProjectID] = append(result[inv.ProjectID], InvoicedPeriod{Start: *inv.PeriodStart, End: *inv.PeriodEnd})
	}
	return result, nil
}

// IsDateInvoiced reports whether date falls within any of the given periods.
func IsDateInvoiced(date time.Time, periods []InvoicedPeriod) bool {
	for _, p := range periods {
		if !date.Before(p.Start) && !date.After(p.End) {
			return true
		}
	}
	return false
}
