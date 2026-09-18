package models

import (
	"time"
)

type Project struct {
	ID          uint     `json:"id" gorm:"primaryKey"`
	Name        string   `json:"name" gorm:"not null" validate:"required,min=1,max=255"`
	Description string   `json:"description" gorm:"type:text"`
	Status      string   `json:"status" gorm:"default:active" validate:"oneof=active completed on-hold cancelled"`
	PricingType string   `json:"pricing_type" validate:"omitempty,oneof=hourly fixed"`
	HourlyRate  *float64 `json:"hourly_rate"`
	FixedPrice  *float64 `json:"fixed_price"`
	// AutoInvoiceEnabled/AutoInvoiceClientID configure unattended monthly
	// pre-invoice notices for hourly projects: when enabled, the scheduler
	// (see services.RunAutoInvoiceNotices) sends a notice e-mail for the
	// previous calendar month to this client on the 1st of every month.
	// Actual invoice creation always requires an explicit approval (the
	// manual "create invoice" button or approving the resulting notice via
	// InvoiceNoticeHandler.ApproveInvoiceNotice), regardless of this flag.
	AutoInvoiceEnabled  bool      `json:"auto_invoice_enabled" gorm:"default:false"`
	AutoInvoiceClientID *uint     `json:"auto_invoice_client_id"`
	CreatedBy           uint      `json:"created_by" gorm:"not null"`
	CreatedByName       string    `json:"created_by_name" gorm:"-"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`

	// Kapcsolat a User modellel
	Creator User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	// Kapcsolat az ügyfelekkel
	Clients []Client `json:"clients,omitempty" gorm:"many2many:project_clients;"`
}

type ProjectCreateRequest struct {
	Name        string   `json:"name" validate:"required,min=1,max=255"`
	Description string   `json:"description"`
	Status      string   `json:"status" validate:"omitempty,oneof=active completed on-hold cancelled"`
	PricingType string   `json:"pricing_type" validate:"omitempty,oneof=hourly fixed"`
	HourlyRate  *float64 `json:"hourly_rate"`
	FixedPrice  *float64 `json:"fixed_price"`
}

type ProjectUpdateRequest struct {
	Name        string   `json:"name" validate:"omitempty,min=1,max=255"`
	Description string   `json:"description"`
	Status      string   `json:"status" validate:"omitempty,oneof=active completed on-hold cancelled"`
	PricingType string   `json:"pricing_type" validate:"omitempty,oneof=hourly fixed"`
	HourlyRate  *float64 `json:"hourly_rate"`
	FixedPrice  *float64 `json:"fixed_price"`
	// AutoInvoiceEnabled is a pointer so "not present in the request" (leave
	// as-is) can be distinguished from an explicit false (turn off).
	AutoInvoiceEnabled  *bool `json:"auto_invoice_enabled"`
	AutoInvoiceClientID *uint `json:"auto_invoice_client_id"`
}

type ProjectResponse struct {
	ID                  uint                    `json:"id"`
	Name                string                  `json:"name"`
	Description         string                  `json:"description"`
	Status              string                  `json:"status"`
	PricingType         string                  `json:"pricing_type"`
	HourlyRate          *float64                `json:"hourly_rate"`
	FixedPrice          *float64                `json:"fixed_price"`
	AutoInvoiceEnabled  bool                    `json:"auto_invoice_enabled"`
	AutoInvoiceClientID *uint                   `json:"auto_invoice_client_id"`
	CreatedBy           uint                    `json:"created_by"`
	CreatedByName       string                  `json:"created_by_name"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
	Clients             []ProjectClientResponse `json:"clients,omitempty"`
}

type ProjectListResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Project  *ProjectResponse  `json:"project,omitempty"`
	Projects []ProjectResponse `json:"projects,omitempty"`
	Count    int               `json:"count,omitempty"`
}
