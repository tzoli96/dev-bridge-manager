// backend/internal/models/email_template.go
package models

import "time"

// ProjectEmailTemplate is a project's custom override for one of the two
// automated e-mail types ("notice" or "ready"). A project with no row for a
// given type falls back to the hardcoded default text (see
// services.BuildInvoiceNoticeText / services.BuildInvoiceReadyText).
type ProjectEmailTemplate struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ProjectID uint      `json:"project_id" gorm:"not null"`
	EmailType string    `json:"email_type" gorm:"size:20;not null"`
	Subject   string    `json:"subject" gorm:"not null"`
	Body      string    `json:"body" gorm:"not null"`
	UpdatedBy uint      `json:"updated_by" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ProjectEmailTemplate) TableName() string { return "project_email_templates" }

// EmailTemplateDTO is what GET /projects/:id/email-templates returns for
// each e-mail type — either the project's saved custom template or the
// hardcoded default, with IsCustom distinguishing which one is in effect.
type EmailTemplateDTO struct {
	EmailType string `json:"email_type"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	IsCustom  bool   `json:"is_custom"`
}

type EmailTemplatesResponse struct {
	Success   bool               `json:"success"`
	Message   string             `json:"message,omitempty"`
	Templates []EmailTemplateDTO `json:"templates,omitempty"`
}

type EmailTemplateSaveRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type EmailTemplateSaveResponse struct {
	Success  bool             `json:"success"`
	Message  string           `json:"message,omitempty"`
	Template EmailTemplateDTO `json:"template"`
}
