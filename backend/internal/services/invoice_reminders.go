// backend/internal/services/invoice_reminders.go
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

// invoiceReminderRepeatInterval matches the approved design: one reminder per
// invoice as soon as it's overdue, repeated every 7 days for as long as it
// stays unpaid.
const invoiceReminderRepeatInterval = 7 * 24 * time.Hour

// invoiceReminderCheckInterval mirrors StartJobScrapingScheduler's daily
// cadence - overdue status only needs to be re-evaluated once a day.
const invoiceReminderCheckInterval = 24 * time.Hour

// BuildInvoiceReminderText composes the Hungarian subject/body for the
// overdue-payment reminder e-mail. Pure function so it can be unit-tested
// without a fake Gmail server, same as BuildInvoiceNoticeText/
// BuildInvoiceReadyText.
func BuildInvoiceReminderText(clientName, projectName, invoiceNumber string, daysOverdue int) (subject, body string) {
	subject = fmt.Sprintf("Fizetési emlékeztető - %s", projectName)
	body = fmt.Sprintf(
		"Kedves %s!\n\nSzeretnénk emlékeztetni, hogy a(z) \"%s\" projekt %s számú számlája %d napja lejárt, és még nem érkezett meg a kiegyenlítése.\n\nÜdvözlettel",
		clientName, projectName, invoiceNumber, daysOverdue,
	)
	return subject, body
}

// reminderIsDue decides whether a new pending InvoiceReminder should be
// proposed for an invoice, given the most recently created reminder for it
// (nil if none exists yet). No reminder is proposed while one is already
// waiting for approval, to avoid stacking duplicates; a sent or dismissed
// reminder is reconsidered once invoiceReminderRepeatInterval has elapsed.
func reminderIsDue(latest *models.InvoiceReminder, now time.Time) bool {
	if latest == nil {
		return true
	}
	if latest.Status == "pending" {
		return false
	}
	return now.Sub(latest.CreatedAt) >= invoiceReminderRepeatInterval
}

// RunInvoiceReminderCheck re-checks Billingo's payment status for every
// overdue, unpaid invoice (the same on-demand call RefreshPaymentStatuses
// uses), then proposes a new pending InvoiceReminder for any invoice that's
// still unpaid and due one per reminderIsDue. It never sends an e-mail
// itself - that only happens once a team member approves the reminder (see
// InvoiceReminderHandler.Approve), mirroring InvoiceNotice's
// propose-then-approve flow.
func RunInvoiceReminderCheck(billingoService *BillingoService) {
	db := database.GetDB()

	var invoices []models.Invoice
	if err := db.
		Where("status = ? AND billingo_invoice_id <> ? AND due_date IS NOT NULL AND due_date < ? AND payment_status <> ?",
			"created", "", time.Now(), "paid").
		Find(&invoices).Error; err != nil {
		log.Printf("⚠️ Invoice reminders: failed to load overdue invoices: %v", err)
		return
	}

	for i := range invoices {
		checkInvoiceForReminder(db, billingoService, &invoices[i])
	}
}

func checkInvoiceForReminder(db *gorm.DB, billingoService *BillingoService, invoice *models.Invoice) {
	status, paidDate, err := billingoService.GetDocumentPaymentStatus(invoice.BillingoInvoiceID)
	if err != nil {
		log.Printf("⚠️ Invoice reminders: failed to refresh payment status for invoice %d: %v", invoice.ID, err)
		return
	}
	if err := db.Model(invoice).Updates(map[string]interface{}{
		"payment_status": status,
		"paid_date":      paidDate,
	}).Error; err != nil {
		log.Printf("⚠️ Invoice reminders: failed to save payment status for invoice %d: %v", invoice.ID, err)
		return
	}
	if status == "paid" {
		return
	}

	var latest models.InvoiceReminder
	err = db.Where("invoice_id = ?", invoice.ID).Order("created_at DESC").First(&latest).Error
	var latestPtr *models.InvoiceReminder
	if err == nil {
		latestPtr = &latest
	}

	if !reminderIsDue(latestPtr, time.Now()) {
		return
	}

	daysOverdue := int(time.Since(*invoice.DueDate).Hours() / 24)
	reminder := models.InvoiceReminder{
		InvoiceID:   invoice.ID,
		ProjectID:   invoice.ProjectID,
		ClientID:    invoice.ClientID,
		DaysOverdue: daysOverdue,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	if err := db.Create(&reminder).Error; err != nil {
		log.Printf("⚠️ Invoice reminders: failed to create reminder for invoice %d: %v", invoice.ID, err)
	}
}

// SendInvoiceReminderEmail sends the approved overdue-payment reminder
// e-mail from the approving user's Gmail account, mirroring
// SendInvoiceReadyEmail. Returns the Gmail message id so the caller can
// persist it onto the InvoiceReminder row.
func SendInvoiceReminderEmail(account models.GmailAccount, client models.Client, project models.Project, invoiceNumber string, daysOverdue int) (string, error) {
	subject, body := BuildInvoiceReminderText(client.Name, project.Name, invoiceNumber, daysOverdue)
	raw := BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "", "", nil)
	return NewRealGmailAPI().SendMessage(context.Background(), &account, raw)
}

// StartInvoiceReminderScheduler mirrors StartJobScrapingScheduler's plain
// time.Ticker pattern: runs once at startup, then once every 24h.
func StartInvoiceReminderScheduler() {
	log.Println("💸 Invoice reminders: running overdue check")
	RunInvoiceReminderCheck(NewBillingoService())

	ticker := time.NewTicker(invoiceReminderCheckInterval)
	for range ticker.C {
		log.Println("💸 Invoice reminders: running overdue check")
		RunInvoiceReminderCheck(NewBillingoService())
	}
}
