// backend/internal/models/invoice_ready_email.go
package models

import "time"

// InvoiceReadyEmail records one send of the invoice-ready e-mail (the
// client-facing e-mail carrying the issued invoice's PDF), so the Billing
// page can show which e-mail went out, when, and to whom for a given
// invoice. Written by services.ApproveInvoiceNotice and the manual
// InvoiceHandler.SendInvoiceEmail after every successful send.
type InvoiceReadyEmail struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	InvoiceID      uint      `json:"invoice_id" gorm:"not null"`
	ProjectID      uint      `json:"project_id" gorm:"not null"`
	ClientID       uint      `json:"client_id" gorm:"not null"`
	SentBy         uint      `json:"sent_by" gorm:"not null"`
	SentAt         time.Time `json:"sent_at"`
	GmailMessageID string    `json:"gmail_message_id" gorm:"size:100;not null"`
}

func (InvoiceReadyEmail) TableName() string { return "invoice_ready_emails" }

// InvoiceReadyEmailWithNames adds the sender's display name for the
// Billing page's per-invoice "E-mail történet" list.
type InvoiceReadyEmailWithNames struct {
	InvoiceReadyEmail
	SentByName string `json:"sent_by_name"`
	ClientName string `json:"client_name"`
}

type InvoiceReadyEmailsResponse struct {
	Success bool                         `json:"success"`
	Message string                       `json:"message,omitempty"`
	Emails  []InvoiceReadyEmailWithNames `json:"emails,omitempty"`
}
