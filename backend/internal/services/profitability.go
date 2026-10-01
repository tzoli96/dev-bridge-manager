// backend/internal/services/profitability.go
package services

import (
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

// LoadOverview gathers the monthly aggregates for the last `months` complete
// months from the existing tables and hands them to the pure BuildOverview.
func LoadOverview(months int) (Overview, error) {
	db := database.GetDB()
	window, from, to := MonthWindow(time.Now(), months)

	var settings models.ProfitSettings
	if err := db.First(&settings, 1).Error; err != nil {
		return Overview{}, err
	}

	invoices, err := loadInvoiceMonths(db, from, to)
	if err != nil {
		return Overview{}, err
	}
	hours, err := loadProjectHoursMonths(db, from, to)
	if err != nil {
		return Overview{}, err
	}
	emails, err := loadEmailMonths(db, from, to)
	if err != nil {
		return Overview{}, err
	}

	var links []ProjectClientLink
	if err := db.Table("project_clients").Select("project_id, client_id").Scan(&links).Error; err != nil {
		return Overview{}, err
	}

	clientNames, err := loadNames(db, "clients")
	if err != nil {
		return Overview{}, err
	}
	projectNames, err := loadNames(db, "projects")
	if err != nil {
		return Overview{}, err
	}

	var allowances []models.ClientMeetingAllowance
	if err := db.Find(&allowances).Error; err != nil {
		return Overview{}, err
	}
	meetingHours := make(map[uint]float64, len(allowances))
	for _, a := range allowances {
		meetingHours[a.ClientID] = a.HoursPerMonth
	}

	return BuildOverview(OverviewInput{
		Months:       window,
		Invoices:     invoices,
		ProjectHours: hours,
		Emails:       emails,
		Links:        links,
		ClientNames:  clientNames,
		ProjectNames: projectNames,
		MeetingHours: meetingHours,
		Settings:     settings,
	}), nil
}

// Same "real invoice" definition as the overdue check (client_health.go):
// status created and an actual Billingo document. Fixed-price invoices may
// have no period_end, so they fall back to the creation date.
func loadInvoiceMonths(db *gorm.DB, from, to time.Time) ([]InvoiceMonth, error) {
	var rows []InvoiceMonth
	err := db.Table("invoices").
		Select("project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM') AS month, SUM(amount) AS amount").
		Where("status = ? AND billingo_invoice_id <> ? AND COALESCE(period_end, created_at) >= ? AND COALESCE(period_end, created_at) < ?",
			"created", "", from, to).
		Group("project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM')").
		Scan(&rows).Error
	return rows, err
}

func loadProjectHoursMonths(db *gorm.DB, from, to time.Time) ([]ProjectHoursMonth, error) {
	var rows []ProjectHoursMonth
	err := db.Table("task_time_entries").
		Select("tasks.project_id AS project_id, to_char(task_time_entries.date, 'YYYY-MM') AS month, SUM(task_time_entries.hours) AS hours").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("task_time_entries.date >= ? AND task_time_entries.date < ?", from, to).
		Group("tasks.project_id, to_char(task_time_entries.date, 'YYYY-MM')").
		Scan(&rows).Error
	return rows, err
}

func loadEmailMonths(db *gorm.DB, from, to time.Time) ([]EmailMonth, error) {
	var rows []EmailMonth
	err := db.Table("emails").
		Select("client_id, project_id, to_char(received_at, 'YYYY-MM') AS month, folder, COUNT(*) AS count").
		Where("client_id IS NOT NULL AND category IN ? AND received_at >= ? AND received_at < ?",
			[]string{models.EmailCategoryClient, models.EmailCategoryBilling}, from, to).
		Group("client_id, project_id, to_char(received_at, 'YYYY-MM'), folder").
		Scan(&rows).Error
	return rows, err
}

func loadNames(db *gorm.DB, table string) (map[uint]string, error) {
	var rows []struct {
		ID   uint
		Name string
	}
	if err := db.Table(table).Select("id, name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	names := make(map[uint]string, len(rows))
	for _, r := range rows {
		names[r.ID] = r.Name
	}
	return names, nil
}
