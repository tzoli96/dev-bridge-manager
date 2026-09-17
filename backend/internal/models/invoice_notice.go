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
}

func (InvoiceNotice) TableName() string { return "invoice_notices" }

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
