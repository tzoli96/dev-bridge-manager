// backend/internal/models/marketing_contact.go
package models

import "time"

type MarketingContact struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Email      string    `json:"email" gorm:"not null;uniqueIndex" validate:"required,email"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Subscribed bool      `json:"subscribed" gorm:"not null;default:true"`
	Tags       string    `json:"tags"`
	Source     string    `json:"source"`
	Notes      string    `json:"notes" gorm:"type:text"`
	CreatedBy  uint      `json:"created_by" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MarketingContactCreateRequest struct {
	Email      string `json:"email" validate:"required,email"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Subscribed *bool  `json:"subscribed"`
	Tags       string `json:"tags"`
	Source     string `json:"source"`
	Notes      string `json:"notes"`
}

type MarketingContactUpdateRequest struct {
	Email      string `json:"email" validate:"required,email"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Subscribed *bool  `json:"subscribed"`
	Tags       string `json:"tags"`
	Source     string `json:"source"`
	Notes      string `json:"notes"`
}

// ImportResult is the response body for POST /marketing-contacts/import.
type ImportResult struct {
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}
