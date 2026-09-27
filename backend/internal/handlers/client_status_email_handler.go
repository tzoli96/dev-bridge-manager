// backend/internal/handlers/client_status_email_handler.go
package handlers

import (
	"context"
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type ClientStatusEmailHandler struct{}

func NewClientStatusEmailHandler() *ClientStatusEmailHandler {
	return &ClientStatusEmailHandler{}
}

// ListClientStatusEmails - GET /api/v1/admin/client-status-emails?status=pending
// - AI-drafted weekly client status emails, for the dashboard widget.
// super_admin only (see routes/client_status_email_routes.go).
func (h *ClientStatusEmailHandler) ListClientStatusEmails(c *fiber.Ctx) error {
	var rows []struct {
		models.ClientStatusEmail
		ClientName string `gorm:"column:client_name"`
	}

	query := database.GetDB().Table("client_status_emails").
		Select("client_status_emails.*, clients.name as client_name").
		Joins("LEFT JOIN clients ON client_status_emails.client_id = clients.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("client_status_emails.status = ?", status)
	}

	if err := query.Order("client_status_emails.created_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(models.ClientStatusEmailListResponse{Success: false, Message: "Error fetching client status emails"})
	}

	emails := make([]models.ClientStatusEmailWithNames, 0, len(rows))
	for _, row := range rows {
		emails = append(emails, models.ClientStatusEmailWithNames{
			ClientStatusEmail: row.ClientStatusEmail,
			ClientName:        row.ClientName,
		})
	}

	return c.JSON(models.ClientStatusEmailListResponse{Success: true, Emails: emails})
}

// Approve - POST /api/v1/admin/client-status-emails/:id/approve - elküldi az
// AI által megírt (opcionálisan szerkesztett) heti státusz-emailt a
// jóváhagyó felhasználó Gmail-fiókjából, és 'sent'-re állítja a rekordot.
func (h *ClientStatusEmailHandler) Approve(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	emailID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Invalid email ID"})
	}

	var req models.ClientStatusEmailApproveRequest
	_ = c.BodyParser(&req)

	db := database.GetDB()

	var draft models.ClientStatusEmail
	if err := db.Where("id = ? AND status = ?", emailID, "pending").First(&draft).Error; err != nil {
		return c.Status(404).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Pending status email not found"})
	}

	var client models.Client
	if err := db.First(&client, draft.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Client not found"})
	}
	if client.Email == "" {
		return c.Status(400).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Client has no email address on file"})
	}

	var account models.GmailAccount
	if err := db.Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		return c.Status(400).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	subject := draft.Subject
	if req.Subject != "" {
		subject = req.Subject
	}
	body := draft.Body
	if req.Body != "" {
		body = req.Body
	}

	raw := services.BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "", "", nil)
	gmailMessageID, err := services.NewRealGmailAPI().SendMessage(context.Background(), &account, raw)
	if err != nil {
		return c.Status(502).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Failed to send e-mail: " + err.Error()})
	}

	sentAt := time.Now()
	if err := db.Model(&draft).Updates(map[string]interface{}{
		"subject":          subject,
		"body":             body,
		"status":           "sent",
		"sent_by":          currentUserID,
		"sent_at":          sentAt,
		"gmail_message_id": gmailMessageID,
	}).Error; err != nil {
		return c.Status(500).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Failed to record sent status email"})
	}
	draft.Subject = subject
	draft.Body = body
	draft.Status = "sent"
	draft.SentBy = &currentUserID
	draft.SentAt = &sentAt
	draft.GmailMessageID = gmailMessageID

	return c.JSON(models.ClientStatusEmailActionResponse{Success: true, Email: &draft})
}

// Dismiss - POST /api/v1/admin/client-status-emails/:id/dismiss - elveti ezt
// a piszkozatot, nem küldi el.
func (h *ClientStatusEmailHandler) Dismiss(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	emailID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Invalid email ID"})
	}

	db := database.GetDB()

	var draft models.ClientStatusEmail
	if err := db.Where("id = ? AND status = ?", emailID, "pending").First(&draft).Error; err != nil {
		return c.Status(404).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Pending status email not found"})
	}

	dismissedAt := time.Now()
	if err := db.Model(&draft).Updates(map[string]interface{}{
		"status":       "dismissed",
		"dismissed_by": currentUserID,
		"dismissed_at": dismissedAt,
	}).Error; err != nil {
		return c.Status(500).JSON(models.ClientStatusEmailActionResponse{Success: false, Message: "Failed to dismiss status email"})
	}
	draft.Status = "dismissed"
	draft.DismissedBy = &currentUserID
	draft.DismissedAt = &dismissedAt

	return c.JSON(models.ClientStatusEmailActionResponse{Success: true, Email: &draft})
}
