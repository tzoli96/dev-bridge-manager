package models

import (
	"time"
)

type Client struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	Type              string    `json:"type" gorm:"default:company" validate:"oneof=company individual"`
	Name              string    `json:"name" gorm:"not null" validate:"required,min=1,max=255"`
	TaxNumber         string    `json:"tax_number" gorm:"size:20"`
	EUVatNumber       string    `json:"eu_vat_number" gorm:"size:30"`
	CompanyRegNumber  string    `json:"company_reg_number" gorm:"size:30"`
	BillingZip        string    `json:"billing_zip" gorm:"size:10"`
	BillingCity       string    `json:"billing_city" gorm:"size:150"`
	BillingAddress    string    `json:"billing_address" gorm:"size:255"`
	BankAccountNumber string    `json:"bank_account_number" gorm:"size:50"`
	Email             string    `json:"email" gorm:"size:255"`
	Phone             string    `json:"phone" gorm:"size:50"`
	Notes             string    `json:"notes" gorm:"type:text"`
	BillingoPartnerID string    `json:"billingo_partner_id" gorm:"column:billingo_partner_id;size:100"`
	IsActive          bool      `json:"is_active" gorm:"default:true"`
	CreatedBy         uint      `json:"created_by" gorm:"not null"`
	CreatedByName     string    `json:"created_by_name" gorm:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Kapcsolat a User modellel
	Creator User `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

type ClientCreateRequest struct {
	Type              string `json:"type" validate:"omitempty,oneof=company individual"`
	Name              string `json:"name" validate:"required,min=1,max=255"`
	TaxNumber         string `json:"tax_number"`
	EUVatNumber       string `json:"eu_vat_number"`
	CompanyRegNumber  string `json:"company_reg_number"`
	BillingZip        string `json:"billing_zip"`
	BillingCity       string `json:"billing_city"`
	BillingAddress    string `json:"billing_address"`
	BankAccountNumber string `json:"bank_account_number"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	Notes             string `json:"notes"`
}

type ClientUpdateRequest struct {
	Type              string `json:"type" validate:"omitempty,oneof=company individual"`
	Name              string `json:"name" validate:"omitempty,min=1,max=255"`
	TaxNumber         string `json:"tax_number"`
	EUVatNumber       string `json:"eu_vat_number"`
	CompanyRegNumber  string `json:"company_reg_number"`
	BillingZip        string `json:"billing_zip"`
	BillingCity       string `json:"billing_city"`
	BillingAddress    string `json:"billing_address"`
	BankAccountNumber string `json:"bank_account_number"`
	Email             string `json:"email"`
	Phone             string `json:"phone"`
	Notes             string `json:"notes"`
	IsActive          *bool  `json:"is_active"`
}

type ClientResponse struct {
	ID                uint      `json:"id"`
	Type              string    `json:"type"`
	Name              string    `json:"name"`
	TaxNumber         string    `json:"tax_number"`
	EUVatNumber       string    `json:"eu_vat_number"`
	CompanyRegNumber  string    `json:"company_reg_number"`
	BillingZip        string    `json:"billing_zip"`
	BillingCity       string    `json:"billing_city"`
	BillingAddress    string    `json:"billing_address"`
	BankAccountNumber string    `json:"bank_account_number"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	Notes             string    `json:"notes"`
	IsActive          bool      `json:"is_active"`
	CreatedBy         uint      `json:"created_by"`
	CreatedByName     string    `json:"created_by_name"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ClientListResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Client  *ClientResponse  `json:"client,omitempty"`
	Clients []ClientResponse `json:"clients,omitempty"`
	Count   int              `json:"count,omitempty"`
}
