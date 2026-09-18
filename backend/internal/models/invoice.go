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
	ItemName              string     `json:"item_name" gorm:"size:255"`
	DueDate               *time.Time `json:"due_date"`
	Amount                float64    `json:"amount"`
	Status                string     `json:"status" gorm:"default:created"`
	// PaymentStatus mirrors Billingo's per-document payment_status
	// ("outstanding", "paid", "partially_paid", "expired", "none") and is
	// only refreshed on demand (see RefreshPaymentStatuses), not pushed by
	// Billingo, since no webhook is configured.
	PaymentStatus string     `json:"payment_status" gorm:"size:20"`
	PaidDate      *time.Time `json:"paid_date"`
	ErrorMessage  string     `json:"error_message" gorm:"type:text"`
	CreatedBy     uint       `json:"created_by" gorm:"not null"`
	CreatedAt     time.Time  `json:"created_at"`

	Client  Client `json:"client,omitempty" gorm:"foreignKey:ClientID"`
	Creator User   `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

type InvoiceCreateRequest struct {
	ClientID    uint   `json:"client_id" validate:"required"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	// ItemName overrides the auto-generated Billingo line-item name
	// ("<project> - fixed price" / "<project> - <start> to <end>") when set.
	ItemName string `json:"item_name"`
	// DueDate (YYYY-MM-DD) overrides the default payment deadline
	// (today + invoiceDueDays) when set.
	DueDate string `json:"due_date"`
	// BaseUnitPrice overrides the pricing-type-derived per-unit rate of the
	// base line item when set (> 0): the hourly rate for hourly projects, or
	// the total price for fixed-price projects. Left unset, the project's
	// configured rate/price is used as before.
	BaseUnitPrice float64 `json:"base_unit_price"`
	// ExtraItems are additional freeform line items (e.g. one-time fees,
	// expenses) added on top of the pricing-type-derived base item.
	ExtraItems []InvoiceItemInput `json:"extra_items"`
}

// InvoiceItemInput is one user-supplied extra line item on an invoice being
// created.
type InvoiceItemInput struct {
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	UnitPrice float64 `json:"unit_price"`
}

// InvoiceItem is one persisted line item of a created invoice (the
// pricing-type-derived base item plus any extras), kept so an invoice's
// full item breakdown can be shown later without recomputing it.
type InvoiceItem struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	InvoiceID     uint      `json:"invoice_id" gorm:"not null;index"`
	Name          string    `json:"name" gorm:"size:255;not null"`
	Quantity      float64   `json:"quantity" gorm:"not null;default:1"`
	Unit          string    `json:"unit" gorm:"size:50"`
	UnitPrice     float64   `json:"unit_price" gorm:"not null"`
	UnitPriceType string    `json:"unit_price_type" gorm:"size:10"`
	LineTotal     float64   `json:"line_total" gorm:"not null"`
	IsBase        bool      `json:"is_base" gorm:"default:false"`
	CreatedAt     time.Time `json:"created_at"`
}

func (InvoiceItem) TableName() string { return "invoice_items" }

type InvoiceResponse struct {
	ID                    uint          `json:"id"`
	ProjectID             uint          `json:"project_id"`
	ProjectName           string        `json:"project_name,omitempty"`
	ClientID              uint          `json:"client_id"`
	ClientName            string        `json:"client_name"`
	BillingoInvoiceID     string        `json:"billingo_invoice_id"`
	BillingoInvoiceNumber string        `json:"billingo_invoice_number"`
	PricingType           string        `json:"pricing_type"`
	PeriodStart           *time.Time    `json:"period_start"`
	PeriodEnd             *time.Time    `json:"period_end"`
	ItemName              string        `json:"item_name"`
	DueDate               *time.Time    `json:"due_date"`
	Amount                float64       `json:"amount"`
	Status                string        `json:"status"`
	PaymentStatus         string        `json:"payment_status"`
	PaidDate              *time.Time    `json:"paid_date"`
	ErrorMessage          string        `json:"error_message"`
	CreatedBy             uint          `json:"created_by"`
	CreatedByName         string        `json:"created_by_name"`
	CreatedAt             time.Time     `json:"created_at"`
	Items                 []InvoiceItem `json:"items"`
}

type InvoiceListResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Invoice  *InvoiceResponse  `json:"invoice,omitempty"`
	Invoices []InvoiceResponse `json:"invoices,omitempty"`
	Count    int               `json:"count,omitempty"`
}

// InvoiceLineItemDTO is one billed time entry, as shown in an hourly
// invoice's breakdown (which hours / which tasks it covers).
type InvoiceLineItemDTO struct {
	TaskID    string  `json:"task_id"`
	TaskTitle string  `json:"task_title"`
	BoardID   uint    `json:"board_id"`
	Date      string  `json:"date"`
	Hours     float64 `json:"hours"`
	UserName  string  `json:"user_name"`
}

type InvoiceBreakdownResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Items   []InvoiceLineItemDTO `json:"items,omitempty"`
}

// MonthlyRevenue is one bucket ("YYYY-MM") of the last 12 calendar months of
// revenue analytics.
type MonthlyRevenue struct {
	Month  string  `json:"month" gorm:"column:month"`
	Amount float64 `json:"amount" gorm:"column:amount"`
}

// YearlyRevenue is one calendar-year ("YYYY") bucket of revenue analytics.
type YearlyRevenue struct {
	Year   string  `json:"year" gorm:"column:year"`
	Amount float64 `json:"amount" gorm:"column:amount"`
}

// RevenueAnalyticsResponse aggregates 'created' invoice amounts (Billingo
// issue date = invoices.created_at) into monthly, yearly, and all-time
// totals, for either a single project or a single client across all its
// projects.
type RevenueAnalyticsResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Monthly []MonthlyRevenue `json:"monthly,omitempty"`
	Yearly  []YearlyRevenue  `json:"yearly,omitempty"`
	Total   float64          `json:"total"`
}
