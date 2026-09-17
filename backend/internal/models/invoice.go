// backend/internal/models/invoice.go
package models

import "time"

type Invoice struct {
	ID                    uint       `json:"id" gorm:"primaryKey"`
	ProjectID             uint       `json:"project_id" gorm:"not null"`
	ClientID              uint       `json:"client_id" gorm:"not null"`
	BillingoInvoiceID     string     `json:"billingo_invoice_id" gorm:"size:50"`
	BillingoInvoiceNumber string     `json:"billingo_invoice_number" gorm:"size:50"`
	PricingType           string     `json:"pricing_type" gorm:"size:20"`
	PeriodStart           *time.Time `json:"period_start"`
	PeriodEnd             *time.Time `json:"period_end"`
	Amount                float64    `json:"amount"`
	Status                string     `json:"status" gorm:"default:created"`
	ErrorMessage          string     `json:"error_message" gorm:"type:text"`
	CreatedBy             uint       `json:"created_by" gorm:"not null"`
	CreatedAt             time.Time  `json:"created_at"`

	Client  Client `json:"client,omitempty" gorm:"foreignKey:ClientID"`
	Creator User   `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

type InvoiceCreateRequest struct {
	ClientID    uint   `json:"client_id" validate:"required"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type InvoiceResponse struct {
	ID                    uint       `json:"id"`
	ProjectID             uint       `json:"project_id"`
	ClientID              uint       `json:"client_id"`
	ClientName            string     `json:"client_name"`
	BillingoInvoiceID     string     `json:"billingo_invoice_id"`
	BillingoInvoiceNumber string     `json:"billingo_invoice_number"`
	PricingType           string     `json:"pricing_type"`
	PeriodStart           *time.Time `json:"period_start"`
	PeriodEnd             *time.Time `json:"period_end"`
	Amount                float64    `json:"amount"`
	Status                string     `json:"status"`
	ErrorMessage          string     `json:"error_message"`
	CreatedBy             uint       `json:"created_by"`
	CreatedByName         string     `json:"created_by_name"`
	CreatedAt             time.Time  `json:"created_at"`
}

type InvoiceListResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Invoice  *InvoiceResponse  `json:"invoice,omitempty"`
	Invoices []InvoiceResponse `json:"invoices,omitempty"`
	Count    int               `json:"count,omitempty"`
}
