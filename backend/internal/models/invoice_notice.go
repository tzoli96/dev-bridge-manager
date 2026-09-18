// backend/internal/models/invoice_notice.go
package models

import "time"

type InvoiceNotice struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	ProjectID      uint       `json:"project_id" gorm:"not null"`
	ClientID       uint       `json:"client_id" gorm:"not null"`
	PeriodStart    *time.Time `json:"period_start"`
	PeriodEnd      *time.Time `json:"period_end"`
	GmailMessageID string     `json:"gmail_message_id" gorm:"size:100;not null"`
	SentBy         uint       `json:"sent_by" gorm:"not null"`
	SentAt         time.Time  `json:"sent_at"`
	Status         string     `json:"status" gorm:"size:20;not null;default:'pending'"`
	InvoiceID      *uint      `json:"invoice_id"`
	ApprovedBy     *uint      `json:"approved_by"`
	ApprovedAt     *time.Time `json:"approved_at"`
}

func (InvoiceNotice) TableName() string { return "invoice_notices" }

// InvoiceNoticeWithNames adds the project/client display names that
// ListAllInvoiceNotices joins in, for the Billing page's pending-notices list.
type InvoiceNoticeWithNames struct {
	InvoiceNotice
	ProjectName string `json:"project_name"`
	ClientName  string `json:"client_name"`
}

type InvoiceNoticeSendRequest struct {
	ClientID    uint   `json:"client_id"`
	PeriodStart string `json:"period_start,omitempty"`
	PeriodEnd   string `json:"period_end,omitempty"`
}

type InvoiceNoticeResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	Notice  *InvoiceNotice `json:"notice,omitempty"`
}

type InvoiceNoticeListResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Notices []InvoiceNotice `json:"notices,omitempty"`
}

// InvoiceNoticeApproveResponse is returned by
// InvoiceNoticeHandler.ApproveInvoiceNotice. EmailSent is false whenever the
// invoice was created successfully but the invoice-ready e-mail could not be
// sent (e.g. the approver has no connected Gmail account) — Message then
// carries a human-readable warning while Success stays true, since the
// invoice itself was created.
type InvoiceNoticeApproveResponse struct {
	Success   bool             `json:"success"`
	Message   string           `json:"message,omitempty"`
	Invoice   *InvoiceResponse `json:"invoice,omitempty"`
	Notice    *InvoiceNotice   `json:"notice,omitempty"`
	EmailSent bool             `json:"email_sent"`
}
