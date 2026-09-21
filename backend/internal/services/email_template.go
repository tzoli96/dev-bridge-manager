// backend/internal/services/email_template.go
package services

import (
	"fmt"
	"strings"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

const (
	EmailTypeNotice = "notice"
	EmailTypeReady  = "ready"
)

// renderEmailTemplate substitutes the {{client_name}}, {{project_name}},
// {{period}}, {{invoice_number}} and {{draft_summary}} placeholders with
// plain string replacement — deliberately not Go's text/template engine, so
// a project's saved subject/body can never execute template actions.
func renderEmailTemplate(text, clientName, projectName, period, invoiceNumber, draftSummary string) string {
	replacer := strings.NewReplacer(
		"{{client_name}}", clientName,
		"{{project_name}}", projectName,
		"{{period}}", period,
		"{{invoice_number}}", invoiceNumber,
		"{{draft_summary}}", draftSummary,
	)
	return replacer.Replace(text)
}

// loadCustomEmailTemplate returns the project's saved template for the given
// type, or nil if none has been configured — the caller then falls back to
// the hardcoded default text.
func loadCustomEmailTemplate(projectID uint, emailType string) *models.ProjectEmailTemplate {
	var tmpl models.ProjectEmailTemplate
	if err := database.GetDB().Where("project_id = ? AND email_type = ?", projectID, emailType).First(&tmpl).Error; err != nil {
		return nil
	}
	return &tmpl
}

func formatPeriod(periodStart, periodEnd *time.Time) string {
	if periodStart == nil || periodEnd == nil {
		return ""
	}
	return fmt.Sprintf("%s - %s", periodStart.Format("2006.01.02"), periodEnd.Format("2006.01.02"))
}

// GetInvoiceNoticeText returns the pre-invoice notice e-mail's subject/body,
// using the project's custom template if one is configured, otherwise the
// hardcoded default (BuildInvoiceNoticeText).
func GetInvoiceNoticeText(projectID uint, clientName, projectName string, periodStart, periodEnd *time.Time, draftSummary string) (subject, body string) {
	tmpl := loadCustomEmailTemplate(projectID, EmailTypeNotice)
	if tmpl == nil {
		return BuildInvoiceNoticeText(clientName, projectName, periodStart, periodEnd, draftSummary)
	}
	period := formatPeriod(periodStart, periodEnd)
	return renderEmailTemplate(tmpl.Subject, clientName, projectName, period, "", draftSummary),
		renderEmailTemplate(tmpl.Body, clientName, projectName, period, "", draftSummary)
}

// GetInvoiceReadyText returns the invoice-ready e-mail's subject/body, using
// the project's custom template if one is configured, otherwise the
// hardcoded default (BuildInvoiceReadyText).
func GetInvoiceReadyText(projectID uint, clientName, projectName, invoiceNumber string) (subject, body string) {
	tmpl := loadCustomEmailTemplate(projectID, EmailTypeReady)
	if tmpl == nil {
		return BuildInvoiceReadyText(clientName, projectName, invoiceNumber)
	}
	return renderEmailTemplate(tmpl.Subject, clientName, projectName, "", invoiceNumber, ""),
		renderEmailTemplate(tmpl.Body, clientName, projectName, "", invoiceNumber, "")
}
