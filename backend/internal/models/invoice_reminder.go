// backend/internal/models/invoice_reminder.go
package models

import "time"

// InvoiceReminder proposes an overdue-payment reminder e-mail for an
// already-issued, still-unpaid invoice. Created by
// services.RunInvoiceReminderCheck once per invoice, then repeated every
// invoiceReminderRepeatInterval as long as the invoice stays unpaid.
// Requires a team member's approval before the e-mail actually goes out
// (see InvoiceReminderHandler.Approve) - mirrors InvoiceNotice's
// propose-then-approve pattern, except approving here only sends an e-mail,
// it never creates an invoice.
type InvoiceReminder struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	InvoiceID      uint       `json:"invoice_id" gorm:"not null"`
	ProjectID      uint       `json:"project_id" gorm:"not null"`
	ClientID       uint       `json:"client_id" gorm:"not null"`
	DaysOverdue    int        `json:"days_overdue" gorm:"not null"`
	Status         string     `json:"status" gorm:"size:20;not null;default:'pending'"`
	CreatedAt      time.Time  `json:"created_at"`
	SentBy         *uint      `json:"sent_by"`
	SentAt         *time.Time `json:"sent_at"`
	GmailMessageID string     `json:"gmail_message_id" gorm:"size:100"`
}

func (InvoiceReminder) TableName() string { return "invoice_reminders" }

// InvoiceReminderWithNames adds the project/client display names and the
// Billingo invoice number that ListPendingInvoiceReminders joins in, for the
// Billing page's pending-reminders list.
type InvoiceReminderWithNames struct {
	InvoiceReminder
	ProjectName           string `json:"project_name"`
	ClientName            string `json:"client_name"`
	BillingoInvoiceNumber string `json:"billingo_invoice_number"`
}

type InvoiceReminderListResponse struct {
	Success   bool                       `json:"success"`
	Message   string                     `json:"message,omitempty"`
	Reminders []InvoiceReminderWithNames `json:"reminders,omitempty"`
}

type InvoiceReminderActionResponse struct {
	Success  bool             `json:"success"`
	Message  string           `json:"message,omitempty"`
	Reminder *InvoiceReminder `json:"reminder,omitempty"`
}
