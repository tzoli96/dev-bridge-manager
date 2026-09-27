// backend/internal/services/client_status_summary.go
package services

import (
	"time"

	"dev-bridge-manager/internal/database"

	"gorm.io/gorm"
)

// ClientWeeklySummary is one client's completed-tasks/hours-logged/invoice-
// activity facts for a [start, end] (inclusive) week, computed by
// ListClientWeeklySummaries and handed to the AI service to draft a status
// email from. Deliberately excludes new-tasks-created and stall flags - the
// user asked for progress/effort/billing signals only.
type ClientWeeklySummary struct {
	ClientID             uint
	ClientName           string
	ClientEmail          string
	CompletedTasks       []string
	HoursLogged          float64
	InvoicesCreated      int
	InvoicesCreatedTotal float64
	InvoicesPaid         int
}

// HasActivity reports whether any signal fired this week - clients with no
// activity at all are skipped, so they don't get an empty status email.
func (s ClientWeeklySummary) HasActivity() bool {
	return len(s.CompletedTasks) > 0 || s.HoursLogged > 0 || s.InvoicesCreated > 0 || s.InvoicesPaid > 0
}

// weekBeforeMonday returns the [start, end] (inclusive) window of the
// calendar week - Monday through Sunday - immediately preceding the Monday
// that now falls on (or, if now isn't a Monday, preceding the most recent
// Monday). Pure function so it's unit-testable without a clock dependency.
func weekBeforeMonday(now time.Time) (time.Time, time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysSinceMonday := (int(today.Weekday()) + 6) % 7
	thisMonday := today.AddDate(0, 0, -daysSinceMonday)
	start := thisMonday.AddDate(0, 0, -7)
	end := thisMonday.AddDate(0, 0, -1)
	return start, end
}

// ListClientWeeklySummaries computes ClientWeeklySummary for every client
// with at least one signal in [start, end] (inclusive calendar days).
func ListClientWeeklySummaries(db *gorm.DB, start, end time.Time) ([]ClientWeeklySummary, error) {
	endExclusive := end.AddDate(0, 0, 1)
	summaries := map[uint]*ClientWeeklySummary{}
	ensure := func(id uint) *ClientWeeklySummary {
		if s, ok := summaries[id]; ok {
			return s
		}
		s := &ClientWeeklySummary{ClientID: id}
		summaries[id] = s
		return s
	}

	var completedRows []struct {
		ClientID uint   `gorm:"column:client_id"`
		Title    string `gorm:"column:title"`
	}
	if err := db.Table("task_activity_log").
		Select("clients.id as client_id, tasks.title as title").
		Joins("JOIN tasks ON tasks.id = task_activity_log.task_id").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Joins("JOIN project_clients ON project_clients.project_id = projects.id").
		Joins("JOIN clients ON clients.id = project_clients.client_id").
		Joins("JOIN task_placements ON task_placements.task_id = tasks.id").
		Joins("JOIN kanban_columns ON kanban_columns.id = task_placements.column_id").
		Where("task_activity_log.event_type = ? AND task_activity_log.created_at >= ? AND task_activity_log.created_at < ? AND kanban_columns.is_done = true AND tasks.is_archived = false",
			"moved", start, endExclusive).
		Order("task_activity_log.created_at ASC").
		Scan(&completedRows).Error; err != nil {
		return nil, err
	}
	for _, r := range completedRows {
		s := ensure(r.ClientID)
		s.CompletedTasks = append(s.CompletedTasks, r.Title)
	}

	var hoursRows []struct {
		ClientID uint    `gorm:"column:client_id"`
		Hours    float64 `gorm:"column:hours"`
	}
	if err := db.Table("task_time_entries").
		Select("clients.id as client_id, COALESCE(SUM(task_time_entries.hours), 0) as hours").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Joins("JOIN project_clients ON project_clients.project_id = projects.id").
		Joins("JOIN clients ON clients.id = project_clients.client_id").
		Where("task_time_entries.date BETWEEN ? AND ?", start, end).
		Group("clients.id").
		Scan(&hoursRows).Error; err != nil {
		return nil, err
	}
	for _, r := range hoursRows {
		s := ensure(r.ClientID)
		s.HoursLogged = r.Hours
	}

	var createdRows []struct {
		ClientID uint    `gorm:"column:client_id"`
		Count    int     `gorm:"column:count"`
		Total    float64 `gorm:"column:total"`
	}
	if err := db.Table("invoices").
		Select("client_id, COUNT(*) as count, COALESCE(SUM(amount), 0) as total").
		Where("created_at >= ? AND created_at < ? AND status = 'created'", start, endExclusive).
		Group("client_id").
		Scan(&createdRows).Error; err != nil {
		return nil, err
	}
	for _, r := range createdRows {
		s := ensure(r.ClientID)
		s.InvoicesCreated = r.Count
		s.InvoicesCreatedTotal = r.Total
	}

	var paidRows []struct {
		ClientID uint `gorm:"column:client_id"`
		Count    int  `gorm:"column:count"`
	}
	if err := db.Table("invoices").
		Select("client_id, COUNT(*) as count").
		Where("paid_date BETWEEN ? AND ? AND status = 'created'", start, end).
		Group("client_id").
		Scan(&paidRows).Error; err != nil {
		return nil, err
	}
	for _, r := range paidRows {
		s := ensure(r.ClientID)
		s.InvoicesPaid = r.Count
	}

	activeIDs := make([]uint, 0, len(summaries))
	for id, s := range summaries {
		if s.HasActivity() {
			activeIDs = append(activeIDs, id)
		}
	}
	if len(activeIDs) == 0 {
		return nil, nil
	}

	var clients []struct {
		ID    uint   `gorm:"column:id"`
		Name  string `gorm:"column:name"`
		Email string `gorm:"column:email"`
	}
	if err := db.Table("clients").Select("id, name, email").Where("id IN ?", activeIDs).Find(&clients).Error; err != nil {
		return nil, err
	}

	result := make([]ClientWeeklySummary, 0, len(clients))
	for _, c := range clients {
		s := summaries[c.ID]
		s.ClientName = c.Name
		s.ClientEmail = c.Email
		result = append(result, *s)
	}
	return result, nil
}

func listClientWeeklySummariesForWeekBeforeNow() ([]ClientWeeklySummary, time.Time, time.Time, error) {
	start, end := weekBeforeMonday(time.Now())
	summaries, err := ListClientWeeklySummaries(database.GetDB(), start, end)
	return summaries, start, end, err
}
