// backend/internal/handlers/email_template_handler.go
package handlers

import (
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type EmailTemplateHandler struct {
	permissionService *services.PermissionService
}

func NewEmailTemplateHandler() *EmailTemplateHandler {
	return &EmailTemplateHandler{
		permissionService: services.NewPermissionService(),
	}
}

func isValidEmailType(emailType string) bool {
	return emailType == services.EmailTypeNotice || emailType == services.EmailTypeReady
}

// defaultEmailTemplateText returns the hardcoded default subject/body for an
// e-mail type, using placeholder tokens instead of real project/client data
// so the UI can show the user what the default looks like before they
// customize it.
func defaultEmailTemplateText(emailType string) (subject, body string) {
	if emailType == services.EmailTypeReady {
		return services.BuildInvoiceReadyText("{{client_name}}", "{{project_name}}", "{{invoice_number}}")
	}
	return services.BuildInvoiceNoticeText("{{client_name}}", "{{project_name}}", nil, nil, "{{draft_summary}}")
}

// GetEmailTemplates - GET /api/v1/projects/:id/email-templates - visszaadja
// mindkét e-mail típus (notice/ready) aktuális szövegét: az egyedi sablont,
// ha van, egyébként az alapértelmezettet.
func (h *EmailTemplateHandler) GetEmailTemplates(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.EmailTemplatesResponse{Success: false, Message: "Invalid project id"})
	}

	var custom []models.ProjectEmailTemplate
	database.GetDB().Where("project_id = ?", projectID).Find(&custom)
	customByType := make(map[string]models.ProjectEmailTemplate, len(custom))
	for _, tmpl := range custom {
		customByType[tmpl.EmailType] = tmpl
	}

	templates := make([]models.EmailTemplateDTO, 0, 2)
	for _, emailType := range []string{services.EmailTypeNotice, services.EmailTypeReady} {
		if tmpl, ok := customByType[emailType]; ok {
			templates = append(templates, models.EmailTemplateDTO{EmailType: emailType, Subject: tmpl.Subject, Body: tmpl.Body, IsCustom: true})
			continue
		}
		subject, body := defaultEmailTemplateText(emailType)
		templates = append(templates, models.EmailTemplateDTO{EmailType: emailType, Subject: subject, Body: body, IsCustom: false})
	}

	return c.JSON(models.EmailTemplatesResponse{Success: true, Templates: templates})
}

// SaveEmailTemplate - PUT /api/v1/projects/:id/email-templates/:type - elmenti
// a projekt egyedi sablonját az adott e-mail típushoz.
func (h *EmailTemplateHandler) SaveEmailTemplate(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.EmailTemplateSaveResponse{Success: false, Message: "Invalid project id"})
	}
	emailType := c.Params("type")
	if !isValidEmailType(emailType) {
		return c.Status(400).JSON(models.EmailTemplateSaveResponse{Success: false, Message: "Invalid email type"})
	}

	var req models.EmailTemplateSaveRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.EmailTemplateSaveResponse{Success: false, Message: "Invalid request body"})
	}
	if req.Subject == "" || req.Body == "" {
		return c.Status(400).JSON(models.EmailTemplateSaveResponse{Success: false, Message: "subject and body are required"})
	}

	db := database.GetDB()
	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.EmailTemplateSaveResponse{Success: false, Message: "Project not found"})
	}

	tmpl := models.ProjectEmailTemplate{
		ProjectID: uint(projectID),
		EmailType: emailType,
		Subject:   req.Subject,
		Body:      req.Body,
		UpdatedBy: currentUserID,
	}
	if err := db.Where("project_id = ? AND email_type = ?", projectID, emailType).
		Assign(models.ProjectEmailTemplate{Subject: req.Subject, Body: req.Body, UpdatedBy: currentUserID}).
		FirstOrCreate(&tmpl).Error; err != nil {
		return c.Status(500).JSON(models.EmailTemplateSaveResponse{Success: false, Message: "Failed to save email template"})
	}

	return c.JSON(models.EmailTemplateSaveResponse{
		Success:  true,
		Template: models.EmailTemplateDTO{EmailType: emailType, Subject: tmpl.Subject, Body: tmpl.Body, IsCustom: true},
	})
}

// DeleteEmailTemplate - DELETE /api/v1/projects/:id/email-templates/:type -
// visszaállítja az alapértelmezett szöveget az adott e-mail típushoz.
func (h *EmailTemplateHandler) DeleteEmailTemplate(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project id"})
	}
	emailType := c.Params("type")
	if !isValidEmailType(emailType) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid email type"})
	}

	database.GetDB().Where("project_id = ? AND email_type = ?", projectID, emailType).Delete(&models.ProjectEmailTemplate{})

	subject, body := defaultEmailTemplateText(emailType)
	return c.JSON(models.EmailTemplateSaveResponse{
		Success:  true,
		Template: models.EmailTemplateDTO{EmailType: emailType, Subject: subject, Body: body, IsCustom: false},
	})
}

// GetInvoiceReadyEmails - GET /api/v1/projects/:id/invoices/:invoiceId/emails
// - az adott számlához tartozó, ténylegesen kiküldött "kész számla" e-mailek
// előzménye (mikor, ki küldte).
func (h *EmailTemplateHandler) GetInvoiceReadyEmails(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceReadyEmailsResponse{Success: false, Message: "Invalid project id"})
	}
	invoiceID, err := strconv.Atoi(c.Params("invoiceId"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceReadyEmailsResponse{Success: false, Message: "Invalid invoice id"})
	}

	invoice, err := loadProjectInvoice(projectID, invoiceID)
	if err != nil {
		return err
	}

	var rows []struct {
		models.InvoiceReadyEmail
		SentByName string `gorm:"column:sent_by_name"`
		ClientName string `gorm:"column:client_name"`
	}
	database.GetDB().Table("invoice_ready_emails").
		Select("invoice_ready_emails.*, users.name as sent_by_name, clients.name as client_name").
		Joins("LEFT JOIN users ON invoice_ready_emails.sent_by = users.id").
		Joins("LEFT JOIN clients ON invoice_ready_emails.client_id = clients.id").
		Where("invoice_ready_emails.invoice_id = ?", invoice.ID).
		Order("invoice_ready_emails.sent_at DESC").
		Scan(&rows)

	emails := make([]models.InvoiceReadyEmailWithNames, 0, len(rows))
	for _, row := range rows {
		emails = append(emails, models.InvoiceReadyEmailWithNames{
			InvoiceReadyEmail: row.InvoiceReadyEmail,
			SentByName:        row.SentByName,
			ClientName:        row.ClientName,
		})
	}

	return c.JSON(models.InvoiceReadyEmailsResponse{Success: true, Emails: emails})
}
