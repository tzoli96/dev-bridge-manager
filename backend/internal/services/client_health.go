// backend/internal/services/client_health.go
package services

import (
	"time"

	"dev-bridge-manager/internal/database"

	"gorm.io/gorm"
)

// ClientHealth is a per-client, computed-on-demand risk indicator combining
// the two already-tracked warning signals in the system: stalled kanban
// tasks and overdue invoices. Unlike KanbanStallFlag/ProjectRenewalFlag it is
// not a persisted, dismissable flag - it always reflects the current data,
// so there's nothing to snooze or clean up.
type ClientHealth struct {
	ClientID          uint   `json:"client_id"`
	ClientName        string `json:"client_name"`
	Status            string `json:"status"`
	HasStalledTask    bool   `json:"has_stalled_task"`
	HasOverdueInvoice bool   `json:"has_overdue_invoice"`
}

// ComputeClientHealth turns the two boolean signals into a traffic-light
// status: red when both fire, yellow when exactly one does, green when
// neither does.
func ComputeClientHealth(hasStalledTask, hasOverdueInvoice bool) string {
	switch {
	case hasStalledTask && hasOverdueInvoice:
		return "red"
	case hasStalledTask || hasOverdueInvoice:
		return "yellow"
	default:
		return "green"
	}
}

// ListClientHealth computes ClientHealth for every client, for the dashboard
// widget and the clients list page.
func ListClientHealth() ([]ClientHealth, error) {
	db := database.GetDB()

	var clients []struct {
		ID   uint   `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := db.Table("clients").Select("id, name").Find(&clients).Error; err != nil {
		return nil, err
	}

	stalledClientIDs, err := clientIDsWithStalledTask(db)
	if err != nil {
		return nil, err
	}

	overdueClientIDs, err := clientIDsWithOverdueInvoice(db)
	if err != nil {
		return nil, err
	}

	health := make([]ClientHealth, 0, len(clients))
	for _, client := range clients {
		hasStalledTask := stalledClientIDs[client.ID]
		hasOverdueInvoice := overdueClientIDs[client.ID]
		health = append(health, ClientHealth{
			ClientID:          client.ID,
			ClientName:        client.Name,
			Status:            ComputeClientHealth(hasStalledTask, hasOverdueInvoice),
			HasStalledTask:    hasStalledTask,
			HasOverdueInvoice: hasOverdueInvoice,
		})
	}

	return health, nil
}

func clientIDsWithStalledTask(db *gorm.DB) (map[uint]bool, error) {
	var clientIDs []uint
	err := db.Table("kanban_stall_flags").
		Select("DISTINCT project_clients.client_id").
		Joins("JOIN project_clients ON project_clients.project_id = kanban_stall_flags.project_id").
		Where("kanban_stall_flags.status = ?", "pending").
		Pluck("project_clients.client_id", &clientIDs).Error
	if err != nil {
		return nil, err
	}
	return toClientIDSet(clientIDs), nil
}

func clientIDsWithOverdueInvoice(db *gorm.DB) (map[uint]bool, error) {
	var clientIDs []uint
	// Same overdue definition as RunInvoiceReminderCheck (invoice_reminders.go).
	err := db.Table("invoices").
		Select("DISTINCT client_id").
		Where("status = ? AND billingo_invoice_id <> ? AND due_date IS NOT NULL AND due_date < ? AND payment_status <> ?",
			"created", "", time.Now(), "paid").
		Pluck("client_id", &clientIDs).Error
	if err != nil {
		return nil, err
	}
	return toClientIDSet(clientIDs), nil
}

func toClientIDSet(ids []uint) map[uint]bool {
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
