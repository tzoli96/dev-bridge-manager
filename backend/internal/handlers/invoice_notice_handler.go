// backend/internal/handlers/invoice_notice_handler.go
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

type InvoiceNoticeHandler struct {
	permissionService *services.PermissionService
	billingoService   *services.BillingoService
}

func NewInvoiceNoticeHandler() *InvoiceNoticeHandler {
	return &InvoiceNoticeHandler{
		permissionService: services.NewPermissionService(),
		billingoService:   services.NewBillingoService(),
	}
}

// SendInvoiceNotice - POST /api/v1/projects/:id/invoice-notice - e-mail
// értesítő küldése az ügyfélnek, a tényleges számla kiállítása előtt.
func (h *InvoiceNoticeHandler) SendInvoiceNotice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid project id"})
	}

	var req models.InvoiceNoticeSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid request body"})
	}
	if req.ClientID == 0 {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "client_id is required"})
	}

	db := database.GetDB()

	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Project not found"})
	}

	var projectClient models.ProjectClient
	if err := db.Where("project_id = ? AND client_id = ?", projectID, req.ClientID).First(&projectClient).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Client is not attached to this project"})
	}

	var client models.Client
	if err := db.First(&client, req.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Client not found"})
	}
	if client.Email == "" {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Client has no email address on file"})
	}

	var account models.GmailAccount
	if err := db.Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	var periodStart, periodEnd *time.Time
	if req.PeriodStart != "" {
		t, err := time.Parse("2006-01-02", req.PeriodStart)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid period_start"})
		}
		periodStart = &t
	}
	if req.PeriodEnd != "" {
		t, err := time.Parse("2006-01-02", req.PeriodEnd)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid period_end"})
		}
		periodEnd = &t
	}

	notice, err := services.SendInvoiceNoticeEmail(project, client, account, periodStart, periodEnd, currentUserID)
	if err != nil {
		return c.Status(502).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Failed to send notice email: " + err.Error()})
	}

	return c.JSON(models.InvoiceNoticeResponse{Success: true, Notice: notice})
}

// ListInvoiceNotices - GET /api/v1/projects/:id/invoice-notices?period_start=&period_end=
func (h *InvoiceNoticeHandler) ListInvoiceNotices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeListResponse{Success: false, Message: "Invalid project id"})
	}

	query := database.GetDB().Where("project_id = ?", projectID)
	if ps := c.Query("period_start"); ps != "" {
		query = query.Where("period_start = ?", ps)
	}
	if pe := c.Query("period_end"); pe != "" {
		query = query.Where("period_end = ?", pe)
	}

	var notices []models.InvoiceNotice
	query.Order("sent_at desc").Find(&notices)

	return c.JSON(models.InvoiceNoticeListResponse{Success: true, Notices: notices})
}

// ApproveInvoiceNotice - POST /api/v1/projects/:id/invoice-notices/:noticeId/approve
// Jóváhagyja a függőben lévő értesítőt: legyártja a tényleges Billingo
// számlát (createInvoiceForProject-tal, ugyanazzal a logikával mint a
// manuális "Számla kiállítása" gomb), majd e-mailben elküldi a PDF-et az
// ügyfélnek. Ha a PDF-küldés bármilyen okból meghiúsul, a számla attól még
// létrejön — csak egy figyelmeztető üzenetet kap vissza a jóváhagyó.
func (h *InvoiceNoticeHandler) ApproveInvoiceNotice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Invalid project id"})
	}
	noticeID, err := strconv.Atoi(c.Params("noticeId"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Invalid notice id"})
	}

	db := database.GetDB()

	var notice models.InvoiceNotice
	if err := db.Where("id = ? AND project_id = ?", noticeID, projectID).First(&notice).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Invoice notice not found"})
	}
	if notice.Status != "pending" {
		return c.Status(409).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Ez az értesítő már jóvá lett hagyva"})
	}

	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Project not found"})
	}
	if project.PricingType == "" {
		return c.Status(400).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Project has no pricing type configured"})
	}

	var client models.Client
	if err := db.First(&client, notice.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Client not found"})
	}

	req := models.InvoiceCreateRequest{ClientID: notice.ClientID}
	if notice.PeriodStart != nil {
		req.PeriodStart = notice.PeriodStart.Format("2006-01-02")
	}
	if notice.PeriodEnd != nil {
		req.PeriodEnd = notice.PeriodEnd.Format("2006-01-02")
	}

	invoice, items, httpStatus, message := createInvoiceForProject(h.billingoService, project, client, req, currentUserID)
	if httpStatus != 0 {
		return c.Status(httpStatus).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: message})
	}

	now := time.Now()
	if err := db.Model(&notice).Updates(map[string]interface{}{
		"status":      "approved",
		"invoice_id":  invoice.ID,
		"approved_by": currentUserID,
		"approved_at": now,
	}).Error; err != nil {
		log.Printf("⚠️ Failed to mark invoice notice %d as approved: %v", notice.ID, err)
	}
	notice.Status = "approved"
	notice.InvoiceID = &invoice.ID
	notice.ApprovedBy = &currentUserID
	notice.ApprovedAt = &now

	emailSent := false
	warning := ""
	var account models.GmailAccount
	if err := db.Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		warning = "A számla elkészült, de nincs csatlakoztatott Gmail-fiókod — kérlek küldd el a PDF-et manuálisan."
	} else {
		var settings models.BillingoSettings
		if err := db.First(&settings, 1).Error; err != nil || settings.APIKey == "" {
			warning = "A számla elkészült, de a Billingo nincs beállítva a PDF letöltéséhez — kérlek küldd el manuálisan."
		} else {
			pdfBytes, err := h.billingoService.DownloadInvoicePDF(settings.APIKey, invoice.BillingoInvoiceID)
			if err != nil {
				log.Printf("⚠️ Failed to download PDF for invoice %d: %v", invoice.ID, err)
				warning = "A számla elkészült, de a PDF letöltése sikertelen — kérlek küldd el manuálisan."
			} else if err := services.SendInvoiceReadyEmail(account, client, project, invoice.BillingoInvoiceNumber, pdfBytes); err != nil {
				log.Printf("⚠️ Failed to send invoice-ready e-mail for invoice %d: %v", invoice.ID, err)
				warning = "A számla elkészült, de a PDF-es e-mail küldése sikertelen — kérlek küldd el manuálisan."
			} else {
				emailSent = true
			}
		}
	}

	response := toInvoiceResponse(*invoice, client.Name, "")
	response.Items = items

	return c.JSON(models.InvoiceNoticeApproveResponse{
		Success:   true,
		Message:   warning,
		Invoice:   &response,
		Notice:    &notice,
		EmailSent: emailSent,
	})
}
