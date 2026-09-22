// backend/internal/handlers/email_handler.go
package handlers

import (
	"fmt"
	"io"
	"log"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type EmailHandler struct {
	gmailAPI     services.GmailAPI
	draftReplier services.DraftReplier
}

func NewEmailHandler() *EmailHandler {
	return &EmailHandler{
		gmailAPI:     services.NewRealGmailAPI(),
		draftReplier: services.NewDraftReplyService(),
	}
}

func currentGmailAccount(userID uint) (*models.GmailAccount, error) {
	var account models.GmailAccount
	if err := database.GetDB().Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func isValidEmailCategory(category string) bool {
	return models.ValidEmailCategories[category]
}

const snippetMaxLen = 200

// generateSnippet derives a Gmail-style preview snippet from a plain-text
// body: whitespace-collapsed, cut to roughly the first 200 characters at a
// word boundary. Used to backfill the sent-mirror row's snippet, which the
// incremental Gmail sync (gmail_sync.go) never revisits once a
// gmail_message_id is known.
func generateSnippet(body string) string {
	joined := strings.Join(strings.Fields(body), " ")
	if len(joined) <= snippetMaxLen {
		return joined
	}
	cut := joined[:snippetMaxLen]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut
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

	category := c.Query("category")
	if category != "" && !isValidEmailCategory(category) {
		return c.Status(400).JSON(models.EmailListResponse{Success: false, Message: "invalid category"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	const pageSize = 25

	var total int64
	db := database.GetDB().Model(&models.Email{}).Where("gmail_account_id = ? AND folder = ?", account.ID, folder)
	if category != "" {
		db = db.Where("category = ?", category)
	}
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
			Category:       e.Category,
		})
	}

	return c.JSON(models.EmailListResponse{Success: true, Emails: items, Total: total})
}

// GetUnreadCount - GET /api/v1/emails/unread-count - olvasatlan levelek
// száma az inbox mappában, a nav sávban lévő piros badge-hez.
func (h *EmailHandler) GetUnreadCount(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.JSON(fiber.Map{"success": true, "count": 0})
	}

	var count int64
	database.GetDB().Model(&models.Email{}).
		Where("gmail_account_id = ? AND folder = 'inbox' AND is_read = ?", account.ID, false).
		Count(&count)

	return c.JSON(fiber.Map{"success": true, "count": count})
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

// DraftReply - POST /api/v1/emails/:id/draft-reply - fetches the email body,
// combines it with the global profile context, and asks the AI service for
// a draft. Returns text only - never saves or sends anything.
func (h *EmailHandler) DraftReply(c *fiber.Ctx) error {
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

	full, err := h.gmailAPI.GetFullMessage(c.Context(), account, email.GmailMessageID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to fetch email body from Gmail: " + err.Error()})
	}

	content := full.BodyText
	if strings.TrimSpace(content) == "" {
		content = full.Subject
	}
	const maxContentLen = 4000
	if len(content) > maxContentLen {
		content = content[:maxContentLen]
	}

	var profile models.Profile
	profileContext := ""
	if err := database.GetDB().Preload("Samples").First(&profile, 1).Error; err == nil {
		profileContext = services.BuildProfileContext(&profile)
	}
	// A missing/unreadable profile row degrades to no persona rather than
	// failing the whole request - drafting a plain reply is still useful.

	similarReplies := services.FindSimilarReplies(c.Context(), database.GetDB(), account, &email, content, h.gmailAPI)

	draft, err := h.draftReplier.DraftReply(c.Context(), content, profileContext, similarReplies)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to generate draft: " + err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "draft": draft})
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
// multipart/form-data: mezők "to", "subject", "body", opcionális
// "in_reply_to_email_id", opcionális "ai_draft_text" (ha a compose egy AI
// válasz-javaslatból indult, visszacsatolás-méréshez), és opcionális
// "files" (max maxAttachmentCount db, max maxAttachmentSize/fájl, csak
// allowedAttachmentMimeTypes típusok).
func (h *EmailHandler) SendEmail(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	to := c.FormValue("to")
	subject := c.FormValue("subject")
	body := c.FormValue("body")
	bodyHTML := c.FormValue("body_html")
	aiDraftText := c.FormValue("ai_draft_text")
	if to == "" || subject == "" {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "to and subject are required"})
	}

	var inReplyToEmailID uint
	if raw := c.FormValue("in_reply_to_email_id"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "Invalid in_reply_to_email_id"})
		}
		inReplyToEmailID = uint(id)
	}

	var attachments []services.MessageAttachment
	if form, err := c.MultipartForm(); err == nil {
		files := form.File["files"]
		if len(files) > maxAttachmentCount {
			return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: fmt.Sprintf("Maximum %d files per email", maxAttachmentCount)})
		}
		for _, fh := range files {
			if fh.Size > maxAttachmentSize {
				return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: fmt.Sprintf("%s exceeds the 10MB size limit", fh.Filename)})
			}
			contentType := fh.Header.Get("Content-Type")
			if !allowedAttachmentMimeTypes[contentType] {
				return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: fmt.Sprintf("%s has an unsupported file type", fh.Filename)})
			}
			src, err := fh.Open()
			if err != nil {
				return c.Status(500).JSON(models.EmailSendResponse{Success: false, Message: fmt.Sprintf("Error reading %s", fh.Filename)})
			}
			data, err := io.ReadAll(src)
			src.Close()
			if err != nil {
				return c.Status(500).JSON(models.EmailSendResponse{Success: false, Message: fmt.Sprintf("Error reading %s", fh.Filename)})
			}
			attachments = append(attachments, services.MessageAttachment{Filename: fh.Filename, ContentType: contentType, Data: data})
		}
	}

	var inReplyToHeader, referencesHeader string
	if inReplyToEmailID != 0 {
		var original models.Email
		if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", inReplyToEmailID, account.ID).First(&original).Error; err == nil {
			full, err := h.gmailAPI.GetFullMessage(c.Context(), account, original.GmailMessageID)
			if err == nil {
				inReplyToHeader = full.MessageIDHeader
				referencesHeader = full.ReferencesHeader
			}
		}
	}

	raw := services.BuildRawMessage(account.EmailAddress, to, subject, body, bodyHTML, inReplyToHeader, referencesHeader, attachments)
	gmailMessageID, err := h.gmailAPI.SendMessage(c.Context(), account, raw)
	if err != nil {
		return c.Status(502).JSON(models.EmailSendResponse{Success: false, Message: "Failed to send email: " + err.Error()})
	}

	// Mirror the sent message locally so it shows up in the Sent tab
	// immediately, rather than waiting for the next scheduled sync.
	now := time.Now()
	attachmentMetas := make([]models.EmailAttachmentMeta, 0, len(attachments))
	for _, a := range attachments {
		attachmentMetas = append(attachmentMetas, models.EmailAttachmentMeta{Filename: a.Filename, Size: int64(len(a.Data))})
	}
	localEmail := models.Email{
		GmailAccountID: account.ID,
		GmailMessageID: gmailMessageID,
		Folder:         "sent",
		FromAddress:    account.EmailAddress,
		ToAddresses:    to,
		Subject:        subject,
		Snippet:        generateSnippet(body),
		HasAttachments: len(attachments) > 0,
		AttachmentMeta: models.AttachmentMetaToJSON(attachmentMetas),
		IsRead:         true,
		ReceivedAt:     now,
		SyncedAt:       now,
	}
	if err := database.GetDB().Create(&localEmail).Error; err != nil {
		log.Printf("email send: failed to mirror sent message %s locally: %v", gmailMessageID, err)
	} else if strings.TrimSpace(aiDraftText) != "" {
		similarity := services.JaccardSimilarity(aiDraftText, body)
		feedback := models.DraftFeedback{
			EmailID:     localEmail.ID,
			AIDraftText: aiDraftText,
			SentText:    body,
			Similarity:  similarity,
		}
		if err := database.GetDB().Create(&feedback).Error; err != nil {
			log.Printf("email send: failed to log draft feedback for email %d: %v", localEmail.ID, err)
		}
	}

	return c.JSON(models.EmailSendResponse{Success: true, GmailMessageID: gmailMessageID})
}
