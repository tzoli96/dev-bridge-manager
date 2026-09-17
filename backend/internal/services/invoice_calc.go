// backend/internal/services/invoice_calc.go
package services

import (
	"dev-bridge-manager/internal/database"
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
