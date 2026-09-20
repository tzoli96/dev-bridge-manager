// backend/internal/handlers/invoice_notice_handler.go
package handlers

import (
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
// számlát (services.ApproveInvoiceNotice-szal, ugyanazzal a logikával mint a
// manuális "Számla kiállítása" gomb és az automatikus jóváhagyás), majd
// e-mailben elküldi a PDF-et az ügyfélnek. Ha a PDF-küldés bármilyen okból
// meghiúsul, a számla attól még létrejön — csak egy figyelmeztető üzenetet
// kap vissza a jóváhagyó.
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

	result, httpStatus, message := services.ApproveInvoiceNotice(notice, project, client, currentUserID, h.billingoService)
	if httpStatus != 0 {
		return c.Status(httpStatus).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: message})
	}

	response := toInvoiceResponse(*result.Invoice, client.Name, "")
	response.Items = result.Items

	return c.JSON(models.InvoiceNoticeApproveResponse{
		Success:   true,
		Message:   result.Warning,
		Invoice:   &response,
		Notice:    &result.Notice,
		EmailSent: result.EmailSent,
	})
}

// ListAllInvoiceNotices - GET /api/v1/invoice-notices?status=pending - minden
// projekt értesítője, opcionális állapot-szűréssel, a Számlázás menüponthoz.
func (h *InvoiceNoticeHandler) ListAllInvoiceNotices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var rows []struct {
		models.InvoiceNotice
		ProjectName           string `gorm:"column:project_name"`
		ClientName            string `gorm:"column:client_name"`
		BillingoInvoiceNumber string `gorm:"column:billingo_invoice_number"`
	}

	query := database.GetDB().Table("invoice_notices").
		Select("invoice_notices.*, projects.name as project_name, clients.name as client_name, invoices.billingo_invoice_number as billingo_invoice_number").
		Joins("LEFT JOIN projects ON invoice_notices.project_id = projects.id").
		Joins("LEFT JOIN clients ON invoice_notices.client_id = clients.id").
		Joins("LEFT JOIN invoices ON invoice_notices.invoice_id = invoices.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("invoice_notices.status = ?", status)
	}

	if err := query.Order("invoice_notices.sent_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error fetching invoice notices"})
	}

	notices := make([]models.InvoiceNoticeWithNames, 0, len(rows))
	for _, row := range rows {
		notices = append(notices, models.InvoiceNoticeWithNames{
			InvoiceNotice:         row.InvoiceNotice,
			ProjectName:           row.ProjectName,
			ClientName:            row.ClientName,
			BillingoInvoiceNumber: row.BillingoInvoiceNumber,
		})
	}

	return c.JSON(fiber.Map{"success": true, "notices": notices})
}
