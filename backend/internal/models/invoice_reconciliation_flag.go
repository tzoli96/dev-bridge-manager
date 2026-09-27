// backend/internal/models/invoice_reconciliation_flag.go
package models

import "time"

// InvoiceReconciliationFlag flags a time-tracking/invoicing mismatch on an
// hourly project. Created by services.RunInvoiceReconciliationCheck,
// surfaced to super_admin users only (see
// routes/invoice_reconciliation_routes.go). Two kinds, distinguished by
// Type:
//   - "discrepancy": an already-invoiced period's currently-logged hours no
//     longer match what was billed (InvoiceID is set).
//   - "stale_hours": the project has logged hours older than the staleness
//     threshold that have never been invoiced at all (InvoiceID is nil).
//
// Dismissing a flag just snoozes it - RunInvoiceReconciliationCheck re-flags
// once invoiceReconciliationRepeatInterval has elapsed and the underlying
// condition still holds.
type InvoiceReconciliationFlag struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	ProjectID   uint       `json:"project_id" gorm:"not null"`
	InvoiceID   *uint      `json:"invoice_id"`
	Type        string     `json:"type" gorm:"size:20;not null"`
	Details     string     `json:"details" gorm:"type:text;not null"`
	Status      string     `json:"status" gorm:"size:20;not null;default:'pending'"`
	CreatedAt   time.Time  `json:"created_at"`
	DismissedBy *uint      `json:"dismissed_by"`
	DismissedAt *time.Time `json:"dismissed_at"`
}

func (InvoiceReconciliationFlag) TableName() string { return "invoice_reconciliation_flags" }

type InvoiceReconciliationFlagWithNames struct {
	InvoiceReconciliationFlag
	ProjectName string `json:"project_name"`
}

type InvoiceReconciliationFlagListResponse struct {
	Success bool                                 `json:"success"`
	Message string                               `json:"message,omitempty"`
	Flags   []InvoiceReconciliationFlagWithNames `json:"flags,omitempty"`
}

type InvoiceReconciliationFlagActionResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message,omitempty"`
	Flag    *InvoiceReconciliationFlag `json:"flag,omitempty"`
}
