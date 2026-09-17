// backend/internal/handlers/email_handler.go
package handlers

import (
	"mime"
	"path/filepath"
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type EmailHandler struct {
	gmailAPI services.GmailAPI
}

func NewEmailHandler() *EmailHandler {
	return &EmailHandler{gmailAPI: services.NewRealGmailAPI()}
}

func currentGmailAccount(userID uint) (*models.GmailAccount, error) {
	var account models.GmailAccount
	if err := database.GetDB().Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// ListEmails - GET /api/v1/emails?folder=inbox|sent&page=
func (h *EmailHandler) ListEmails(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailListResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	folder := c.Query("folder", "inbox")
	if folder != "inbox" && folder != "sent" {
		return c.Status(400).JSON(models.EmailListResponse{Success: false, Message: "folder must be 'inbox' or 'sent'"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	const pageSize = 25

	var total int64
	db := database.GetDB().Model(&models.Email{}).Where("gmail_account_id = ? AND folder = ?", account.ID, folder)
	db.Count(&total)

	var rows []models.Email
	db.Order("received_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows)

	items := make([]models.EmailListItem, 0, len(rows))
	for _, e := range rows {
		items = append(items, models.EmailListItem{
			ID:             e.ID,
			Folder:         e.Folder,
			FromAddress:    e.FromAddress,
			FromName:       e.FromName,
			ToAddresses:    e.ToAddresses,
			Subject:        e.Subject,
			Snippet:        e.Snippet,
			HasAttachments: e.HasAttachments,
			Attachments:    e.Attachments(),
			IsRead:         e.IsRead,
			ReceivedAt:     e.ReceivedAt,
		})
	}

	return c.JSON(models.EmailListResponse{Success: true, Emails: items, Total: total})
}

// GetEmail - GET /api/v1/emails/:id - helyi metaadat + élő body lekérés
// a Gmailtől (a body sosem kerül tárolásra).
func (h *EmailHandler) GetEmail(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailDetailResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.EmailDetailResponse{Success: false, Message: "Invalid email id"})
	}

	var email models.Email
	if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", id, account.ID).First(&email).Error; err != nil {
		return c.Status(404).JSON(models.EmailDetailResponse{Success: false, Message: "Email not found"})
	}

	full, err := h.gmailAPI.GetFullMessage(c.Context(), account, email.GmailMessageID)
	if err != nil {
		return c.Status(502).JSON(models.EmailDetailResponse{Success: false, Message: "Failed to fetch email body from Gmail: " + err.Error()})
	}

	if !email.IsRead {
		database.GetDB().Model(&email).Update("is_read", true)
	}

	return c.JSON(models.EmailDetailResponse{
		Success:     true,
		ID:          email.ID,
		Folder:      email.Folder,
		Subject:     full.Subject,
		From:        full.FromAddress,
		To:          full.ToAddresses,
		BodyText:    full.BodyText,
		BodyHTML:    full.BodyHTML,
		Attachments: full.Attachments,
		ReceivedAt:  full.ReceivedAt,
	})
}

// GetAttachment - GET /api/v1/emails/:id/attachments/:attachmentId - proxyzott
// letöltés: a Gmail token sosem jut el a böngészőhöz.
func (h *EmailHandler) GetAttachment(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Connect your Gmail account first"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid email id"})
	}

	var email models.Email
	if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", id, account.ID).First(&email).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Email not found"})
	}

	attachmentID := c.Params("attachmentId")
	var filename string
	for _, a := range email.Attachments() {
		if a.AttachmentID == attachmentID {
			filename = a.Filename
			break
		}
	}
	if filename == "" {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Attachment not found on this email"})
	}

	data, err := h.gmailAPI.GetAttachment(c.Context(), account, email.GmailMessageID, attachmentID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to fetch attachment from Gmail: " + err.Error()})
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	return c.Send(data)
}

// SendEmail - POST /api/v1/emails/send - új levél vagy válasz küldése
func (h *EmailHandler) SendEmail(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	var req models.EmailSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "Invalid request body"})
	}
	if req.To == "" || req.Subject == "" {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "to and subject are required"})
	}

	var inReplyToHeader, referencesHeader string
	if req.InReplyToEmailID != 0 {
		var original models.Email
		if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", req.InReplyToEmailID, account.ID).First(&original).Error; err == nil {
			full, err := h.gmailAPI.GetFullMessage(c.Context(), account, original.GmailMessageID)
			if err == nil {
				inReplyToHeader = full.MessageIDHeader
				referencesHeader = full.ReferencesHeader
			}
		}
	}

	raw := services.BuildRawMessage(account.EmailAddress, req.To, req.Subject, req.Body, inReplyToHeader, referencesHeader)
	gmailMessageID, err := h.gmailAPI.SendMessage(c.Context(), account, raw)
	if err != nil {
		return c.Status(502).JSON(models.EmailSendResponse{Success: false, Message: "Failed to send email: " + err.Error()})
	}

	return c.JSON(models.EmailSendResponse{Success: true, GmailMessageID: gmailMessageID})
}
