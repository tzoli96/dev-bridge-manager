// backend/internal/services/auto_invoice.go
package services

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"log"
	"time"
)

// RunAutoInvoiceNotices sends a pre-invoice notice e-mail (never creates the
// invoice itself) for the previous calendar month, for every hourly project
// with auto-invoicing enabled, to its configured client. Actual invoice
// creation only happens once a team member approves the resulting notice via
// InvoiceNoticeHandler.ApproveInvoiceNotice.
//
// It is safe to call more than once on the same day (see
// StartAutoInvoiceScheduler): each project is skipped if a notice or invoice
// already exists for that exact project/period, so re-running never sends a
// duplicate notice.
func RunAutoInvoiceNotices() {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-24 * time.Hour)

	var projects []models.Project
	if err := database.GetDB().
		Where("auto_invoice_enabled = ? AND pricing_type = ? AND auto_invoice_client_id IS NOT NULL", true, "hourly").
		Find(&projects).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to load eligible projects: %v", err)
		return
	}

	for _, project := range projects {
		autoNotifyProject(project, periodStart, periodEnd)
	}
}

func autoNotifyProject(project models.Project, periodStart, periodEnd time.Time) {
	db := database.GetDB()

	var existingNotice models.InvoiceNotice
	if err := db.Where(
		"project_id = ? AND client_id = ? AND period_start = ? AND period_end = ?",
		project.ID, *project.AutoInvoiceClientID, periodStart, periodEnd,
	).First(&existingNotice).Error; err == nil {
		return // notice already sent for this project/period
	}

	var existingInvoice models.Invoice
	if err := db.Where(
		"project_id = ? AND period_start = ? AND period_end = ? AND status IN ('created','pending')",
		project.ID, periodStart, periodEnd,
	).First(&existingInvoice).Error; err == nil {
		return // already invoiced (e.g. via the manual button) for this period
	}

	if project.HourlyRate == nil {
		log.Printf("⚠️ Auto-invoicing: project %d has no hourly rate configured, skipping", project.ID)
		return
	}

	var client models.Client
	if err := db.First(&client, *project.AutoInvoiceClientID).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: project %d's configured client %d not found, skipping", project.ID, *project.AutoInvoiceClientID)
		return
	}

	totalHours, err := SumLoggedHours(project.ID, periodStart, periodEnd)
	if err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to sum hours for project %d: %v", project.ID, err)
		return
	}
	if totalHours <= 0 {
		log.Printf("ℹ️ Auto-invoicing: project %d has no logged hours for %s, skipping", project.ID, periodStart.Format("2006-01"))
		return
	}

	var account models.GmailAccount
	if err := db.Where("user_id = ?", project.CreatedBy).First(&account).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: project %d's creator (user %d) has no connected Gmail account, skipping notice", project.ID, project.CreatedBy)
		return
	}

	notice, err := SendInvoiceNoticeEmail(project, client, account, &periodStart, &periodEnd, project.CreatedBy)
	if err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to send notice for project %d: %v", project.ID, err)
		return
	}

	log.Printf("✅ Auto-invoicing: sent notice for project %d (%.2f óra)", project.ID, totalHours)

	if !project.AutoInvoiceAutoApprove {
		return
	}

	billingoService := NewBillingoService()
	if _, httpStatus, message := ApproveInvoiceNotice(*notice, project, client, project.CreatedBy, billingoService); httpStatus != 0 {
		log.Printf("⚠️ Auto-invoicing: auto-approve failed for project %d notice %d: %s", project.ID, notice.ID, message)
		return
	}

	log.Printf("✅ Auto-invoicing: auto-approved notice %d for project %d", notice.ID, project.ID)
}
