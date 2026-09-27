// backend/internal/handlers/invoice_reminder_handler.go
package handlers

import (
	"log"
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type InvoiceReminderHandler struct {
	permissionService *services.PermissionService
}

func NewInvoiceReminderHandler() *InvoiceReminderHandler {
	return &InvoiceReminderHandler{
		permissionService: services.NewPermissionService(),
	}
}

// ListInvoiceReminders - GET /api/v1/invoice-reminders?status=pending - minden
// emlékeztető, opcionális állapot-szűréssel, a Számlázás menüponthoz.
func (h *InvoiceReminderHandler) ListInvoiceReminders(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var rows []struct {
		models.InvoiceReminder
		ProjectName           string `gorm:"column:project_name"`
		ClientName            string `gorm:"column:client_name"`
		BillingoInvoiceNumber string `gorm:"column:billingo_invoice_number"`
	}

	query := database.GetDB().Table("invoice_reminders").
		Select("invoice_reminders.*, projects.name as project_name, clients.name as client_name, invoices.billingo_invoice_number as billingo_invoice_number").
		Joins("LEFT JOIN projects ON invoice_reminders.project_id = projects.id").
		Joins("LEFT JOIN clients ON invoice_reminders.client_id = clients.id").
		Joins("LEFT JOIN invoices ON invoice_reminders.invoice_id = invoices.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("invoice_reminders.status = ?", status)
	}

	if err := query.Order("invoice_reminders.created_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(models.InvoiceReminderListResponse{Success: false, Message: "Error fetching invoice reminders"})
	}

	reminders := make([]models.InvoiceReminderWithNames, 0, len(rows))
	for _, row := range rows {
		reminders = append(reminders, models.InvoiceReminderWithNames{
			InvoiceReminder:       row.InvoiceReminder,
			ProjectName:           row.ProjectName,
			ClientName:            row.ClientName,
			BillingoInvoiceNumber: row.BillingoInvoiceNumber,
		})
	}

	return c.JSON(models.InvoiceReminderListResponse{Success: true, Reminders: reminders})
}

// Approve - POST /api/v1/invoice-reminders/:id/approve - elküldi az
// emlékeztető e-mailt a jóváhagyó felhasználó Gmail-fiókjából, és 'sent'-re
// állítja a rekordot.
func (h *InvoiceReminderHandler) Approve(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	reminderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Invalid reminder ID"})
	}

	db := database.GetDB()

	var reminder models.InvoiceReminder
	if err := db.Where("id = ? AND status = ?", reminderID, "pending").First(&reminder).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Pending reminder not found"})
	}

	var invoice models.Invoice
	if err := db.First(&invoice, reminder.InvoiceID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Invoice not found"})
	}
	var project models.Project
	if err := db.First(&project, reminder.ProjectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Project not found"})
	}
	var client models.Client
	if err := db.First(&client, reminder.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Client not found"})
	}
	if client.Email == "" {
		return c.Status(400).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Client has no email address on file"})
	}

	var account models.GmailAccount
	if err := db.Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	gmailMessageID, err := services.SendInvoiceReminderEmail(account, client, project, invoice.BillingoInvoiceNumber, reminder.DaysOverdue)
	if err != nil {
		return c.Status(502).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Failed to send e-mail: " + err.Error()})
	}

	sentAt := time.Now()
	if err := db.Model(&reminder).Updates(map[string]interface{}{
		"status":           "sent",
		"sent_by":          currentUserID,
		"sent_at":          sentAt,
		"gmail_message_id": gmailMessageID,
	}).Error; err != nil {
		log.Printf("⚠️ Failed to record sent invoice reminder %d: %v", reminder.ID, err)
	}
	reminder.Status = "sent"
	reminder.SentBy = &currentUserID
	reminder.SentAt = &sentAt
	reminder.GmailMessageID = gmailMessageID

	return c.JSON(models.InvoiceReminderActionResponse{Success: true, Reminder: &reminder})
}

// Dismiss - POST /api/v1/invoice-reminders/:id/dismiss - kihagyja ezt a
// kört (pl. már manuálisan intézve lett); a scheduler a következő
// invoiceReminderRepeatInterval elteltével újra megnézi, ha a számla még
// mindig lejárt és kifizetetlen.
func (h *InvoiceReminderHandler) Dismiss(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	reminderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Invalid reminder ID"})
	}

	db := database.GetDB()

	var reminder models.InvoiceReminder
	if err := db.Where("id = ? AND status = ?", reminderID, "pending").First(&reminder).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Pending reminder not found"})
	}

	if err := db.Model(&reminder).Update("status", "dismissed").Error; err != nil {
		return c.Status(500).JSON(models.InvoiceReminderActionResponse{Success: false, Message: "Failed to dismiss reminder"})
	}
	reminder.Status = "dismissed"

	return c.JSON(models.InvoiceReminderActionResponse{Success: true, Reminder: &reminder})
}
