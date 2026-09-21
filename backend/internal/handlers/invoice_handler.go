// backend/internal/handlers/invoice_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

type InvoiceHandler struct {
	permissionService *services.PermissionService
	billingoService   *services.BillingoService
}

func NewInvoiceHandler() *InvoiceHandler {
	return &InvoiceHandler{
		permissionService: services.NewPermissionService(),
		billingoService:   services.NewBillingoService(),
	}
}

// checkInvoiceAccess - jogosultság ellenőrzése egy adott invoices.* jogra,
// admin/super_admin/manager fallback-kel. Package-level (not a method) so
// invoice_notice_handler.go can reuse the exact same rule without
// duplicating it or depending on an *InvoiceHandler instance.
func checkInvoiceAccess(ps *services.PermissionService, userID uint, action string) error {
	hasPermission, err := ps.CheckUserPermission(userID, action)
	if err != nil || !hasPermission {
		user, err := ps.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" && user.Role.Name != "manager" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

func toInvoiceResponse(inv models.Invoice, clientName, createdByName string) models.InvoiceResponse {
	return models.InvoiceResponse{
		ID:                    inv.ID,
		ProjectID:             inv.ProjectID,
		ClientID:              inv.ClientID,
		ClientName:            clientName,
		BillingoInvoiceID:     inv.BillingoInvoiceID,
		BillingoInvoiceNumber: inv.BillingoInvoiceNumber,
		PricingType:           inv.PricingType,
		PeriodStart:           inv.PeriodStart,
		PeriodEnd:             inv.PeriodEnd,
		ItemName:              inv.ItemName,
		DueDate:               inv.DueDate,
		Amount:                inv.Amount,
		Status:                inv.Status,
		PaymentStatus:         inv.PaymentStatus,
		PaidDate:              inv.PaidDate,
		ErrorMessage:          inv.ErrorMessage,
		CreatedBy:             inv.CreatedBy,
		CreatedByName:         createdByName,
		CreatedAt:             inv.CreatedAt,
	}
}

func ptrInvoiceResponse(r models.InvoiceResponse) *models.InvoiceResponse { return &r }

// CreateInvoice - POST /api/v1/projects/:id/invoices
func (h *InvoiceHandler) CreateInvoice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid project ID"})
	}

	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceListResponse{Success: false, Message: "Project not found"})
	}

	if project.PricingType == "" {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no pricing type configured"})
	}
	if project.PricingType == "hobby" {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Hobbi projektre nem lehet számlázni"})
	}

	var req models.InvoiceCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid request body"})
	}

	if req.ClientID == 0 {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Client ID is required"})
	}

	var projectClient models.ProjectClient
	if err := database.GetDB().Where("project_id = ? AND client_id = ?", projectID, req.ClientID).First(&projectClient).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Client is not attached to this project"})
	}

	var client models.Client
	if err := database.GetDB().First(&client, req.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceListResponse{Success: false, Message: "Client not found"})
	}

	invoice, items, httpStatus, message := services.CreateInvoiceForProject(h.billingoService, project, client, req, currentUserID)
	if httpStatus != 0 {
		return c.Status(httpStatus).JSON(models.InvoiceListResponse{Success: false, Message: message})
	}

	response := toInvoiceResponse(*invoice, client.Name, "")
	response.Items = items

	return c.Status(201).JSON(models.InvoiceListResponse{
		Success: true,
		Message: "Invoice created successfully",
		Invoice: ptrInvoiceResponse(response),
	})
}

// getRevenueAnalytics aggregates 'created' invoice amounts filtered by a
// single column (either "project_id" or "client_id", always a fixed literal
// supplied by the caller below, never user input) into the last 12 calendar
// months, all calendar years, and an all-time total. Shared by the project-
// and client-scoped analytics endpoints so the aggregation logic exists once.
func getRevenueAnalytics(filterColumn string, filterValue uint) (models.RevenueAnalyticsResponse, error) {
	db := database.GetDB()
	whereClause := filterColumn + " = ? AND status = 'created'"

	var monthly []models.MonthlyRevenue
	if err := db.Table("invoices").
		Select("to_char(created_at, 'YYYY-MM') as month, COALESCE(SUM(amount), 0) as amount").
		Where(whereClause+" AND created_at >= ?", filterValue, time.Now().AddDate(0, -11, 0)).
		Group("month").
		Order("month ASC").
		Scan(&monthly).Error; err != nil {
		return models.RevenueAnalyticsResponse{}, err
	}

	var yearly []models.YearlyRevenue
	if err := db.Table("invoices").
		Select("to_char(created_at, 'YYYY') as year, COALESCE(SUM(amount), 0) as amount").
		Where(whereClause, filterValue).
		Group("year").
		Order("year ASC").
		Scan(&yearly).Error; err != nil {
		return models.RevenueAnalyticsResponse{}, err
	}

	var total float64
	if err := db.Table("invoices").
		Select("COALESCE(SUM(amount), 0)").
		Where(whereClause, filterValue).
		Scan(&total).Error; err != nil {
		return models.RevenueAnalyticsResponse{}, err
	}

	return models.RevenueAnalyticsResponse{Success: true, Monthly: monthly, Yearly: yearly, Total: total}, nil
}

// GetProjectRevenueAnalytics - GET /api/v1/projects/:id/invoices/analytics
func (h *InvoiceHandler) GetProjectRevenueAnalytics(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.RevenueAnalyticsResponse{Success: false, Message: "Invalid project ID"})
	}

	resp, err := getRevenueAnalytics("project_id", uint(projectID))
	if err != nil {
		return c.Status(500).JSON(models.RevenueAnalyticsResponse{Success: false, Message: "Error computing revenue analytics"})
	}
	return c.JSON(resp)
}

// GetClientRevenueAnalytics - GET /api/v1/clients/:id/invoices/analytics
func (h *InvoiceHandler) GetClientRevenueAnalytics(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	clientID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.RevenueAnalyticsResponse{Success: false, Message: "Invalid client ID"})
	}

	resp, err := getRevenueAnalytics("client_id", uint(clientID))
	if err != nil {
		return c.Status(500).JSON(models.RevenueAnalyticsResponse{Success: false, Message: "Error computing revenue analytics"})
	}
	return c.JSON(resp)
}

// GetProjectInvoices - GET /api/v1/projects/:id/invoices
func (h *InvoiceHandler) GetProjectInvoices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid project ID"})
	}

	var rows []struct {
		models.Invoice
		ClientName    string `gorm:"column:client_name"`
		CreatedByName string `gorm:"column:created_by_name"`
	}

	err = database.GetDB().Table("invoices").
		Select("invoices.*, clients.name as client_name, users.name as created_by_name").
		Joins("LEFT JOIN clients ON invoices.client_id = clients.id").
		Joins("LEFT JOIN users ON invoices.created_by = users.id").
		Where("invoices.project_id = ?", projectID).
		Order("invoices.created_at DESC").
		Scan(&rows).Error

	if err != nil {
		return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Error fetching invoices"})
	}

	invoiceIDs := make([]uint, len(rows))
	for i, row := range rows {
		invoiceIDs[i] = row.ID
	}
	itemsByInvoice := loadInvoiceItemsByInvoiceIDs(invoiceIDs)

	response := make([]models.InvoiceResponse, 0, len(rows))
	for _, row := range rows {
		r := toInvoiceResponse(row.Invoice, row.ClientName, row.CreatedByName)
		r.Items = itemsByInvoice[row.ID]
		response = append(response, r)
	}

	return c.JSON(models.InvoiceListResponse{
		Success:  true,
		Message:  "Invoices retrieved successfully",
		Invoices: response,
		Count:    len(response),
	})
}

// ListAllInvoices - GET /api/v1/invoices - minden invoice, projektenkénti
// szűréssel (opcionális ?project_id=), a Számlázás menüponthoz.
func (h *InvoiceHandler) ListAllInvoices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	var rows []struct {
		models.Invoice
		ProjectName   string `gorm:"column:project_name"`
		ClientName    string `gorm:"column:client_name"`
		CreatedByName string `gorm:"column:created_by_name"`
	}

	query := database.GetDB().Table("invoices").
		Select("invoices.*, projects.name as project_name, clients.name as client_name, users.name as created_by_name").
		Joins("LEFT JOIN projects ON invoices.project_id = projects.id").
		Joins("LEFT JOIN clients ON invoices.client_id = clients.id").
		Joins("LEFT JOIN users ON invoices.created_by = users.id")

	if projectIDParam := c.Query("project_id"); projectIDParam != "" {
		projectID, err := strconv.Atoi(projectIDParam)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid project_id"})
		}
		query = query.Where("invoices.project_id = ?", projectID)
	}

	if err := query.Order("invoices.created_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Error fetching invoices"})
	}

	invoiceIDs := make([]uint, len(rows))
	for i, row := range rows {
		invoiceIDs[i] = row.ID
	}
	itemsByInvoice := loadInvoiceItemsByInvoiceIDs(invoiceIDs)

	response := make([]models.InvoiceResponse, 0, len(rows))
	for _, row := range rows {
		r := toInvoiceResponse(row.Invoice, row.ClientName, row.CreatedByName)
		r.ProjectName = row.ProjectName
		r.Items = itemsByInvoice[row.ID]
		response = append(response, r)
	}

	return c.JSON(models.InvoiceListResponse{
		Success:  true,
		Message:  "Invoices retrieved successfully",
		Invoices: response,
		Count:    len(response),
	})
}

// loadInvoiceItemsByInvoiceIDs batch-loads invoice_items rows for a set of
// invoice ids, keyed by invoice id.
func loadInvoiceItemsByInvoiceIDs(invoiceIDs []uint) map[uint][]models.InvoiceItem {
	result := make(map[uint][]models.InvoiceItem)
	if len(invoiceIDs) == 0 {
		return result
	}
	var items []models.InvoiceItem
	database.GetDB().Where("invoice_id IN ?", invoiceIDs).Order("id ASC").Find(&items)
	for _, item := range items {
		result[item.InvoiceID] = append(result[item.InvoiceID], item)
	}
	return result
}

// loadProjectInvoice loads an invoice scoped to a project, returning a 404
// fiber error if it doesn't exist under that project.
func loadProjectInvoice(projectID, invoiceID int) (*models.Invoice, error) {
	var invoice models.Invoice
	if err := database.GetDB().Where("id = ? AND project_id = ?", invoiceID, projectID).First(&invoice).Error; err != nil {
		return nil, fiber.NewError(404, "Invoice not found")
	}
	return &invoice, nil
}

// GetInvoiceBreakdown - GET /api/v1/projects/:id/invoices/:invoiceId/breakdown
// Returns the individual logged hours/tasks an hourly invoice covered.
func (h *InvoiceHandler) GetInvoiceBreakdown(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceBreakdownResponse{Success: false, Message: "Invalid project ID"})
	}
	invoiceID, err := strconv.Atoi(c.Params("invoiceId"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceBreakdownResponse{Success: false, Message: "Invalid invoice ID"})
	}

	invoice, err := loadProjectInvoice(projectID, invoiceID)
	if err != nil {
		return err
	}
	if invoice.PricingType != "hourly" || invoice.PeriodStart == nil || invoice.PeriodEnd == nil {
		return c.JSON(models.InvoiceBreakdownResponse{Success: true, Message: "No hourly breakdown for this invoice", Items: []models.InvoiceLineItemDTO{}})
	}

	lines, err := services.GetHourlyLineItems(uint(projectID), *invoice.PeriodStart, *invoice.PeriodEnd)
	if err != nil {
		return c.Status(500).JSON(models.InvoiceBreakdownResponse{Success: false, Message: "Error loading invoice breakdown"})
	}

	items := make([]models.InvoiceLineItemDTO, 0, len(lines))
	for _, l := range lines {
		items = append(items, models.InvoiceLineItemDTO{
			TaskID:    strconv.FormatUint(uint64(l.TaskID), 10),
			TaskTitle: l.TaskTitle,
			BoardID:   l.BoardID,
			Date:      l.Date.Format("2006-01-02"),
			Hours:     l.Hours,
			UserName:  l.UserName,
		})
	}

	return c.JSON(models.InvoiceBreakdownResponse{Success: true, Message: "Invoice breakdown retrieved successfully", Items: items})
}

// DownloadInvoicePDF - GET /api/v1/projects/:id/invoices/:invoiceId/pdf
// Proxies Billingo's document download so the API key never reaches the browser.
func (h *InvoiceHandler) DownloadInvoicePDF(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	invoiceID, err := strconv.Atoi(c.Params("invoiceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid invoice ID"})
	}

	invoice, err := loadProjectInvoice(projectID, invoiceID)
	if err != nil {
		return err
	}
	if invoice.BillingoInvoiceID == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "This invoice was never created in Billingo"})
	}

	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil || settings.APIKey == "" {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Billingo is not configured"})
	}

	pdfBytes, err := h.billingoService.DownloadInvoicePDF(settings.APIKey, invoice.BillingoInvoiceID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Billingo error: " + err.Error()})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s.pdf\"", invoice.BillingoInvoiceNumber))
	return c.Send(pdfBytes)
}

// SendInvoiceEmail - POST /api/v1/projects/:id/invoices/:invoiceId/send-email
// Manually (re-)sends the already-issued invoice's PDF to the client from the
// current user's Gmail account — the same e-mail services.ApproveInvoiceNotice
// sends automatically, exposed here for invoices created via the manual
// "Számla kiállítása" button, or for a retry after that automatic send failed.
func (h *InvoiceHandler) SendInvoiceEmail(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	invoiceID, err := strconv.Atoi(c.Params("invoiceId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid invoice ID"})
	}

	invoice, err := loadProjectInvoice(projectID, invoiceID)
	if err != nil {
		return err
	}
	if invoice.Status != "created" || invoice.BillingoInvoiceID == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "This invoice was never created in Billingo"})
	}

	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Project not found"})
	}
	var client models.Client
	if err := database.GetDB().First(&client, invoice.ClientID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Client not found"})
	}
	if client.Email == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Client has no email address on file"})
	}

	var account models.GmailAccount
	if err := database.GetDB().Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Connect your Gmail account first"})
	}

	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil || settings.APIKey == "" {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Billingo is not configured"})
	}

	pdfBytes, err := h.billingoService.DownloadInvoicePDF(settings.APIKey, invoice.BillingoInvoiceID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Billingo error: " + err.Error()})
	}

	gmailMessageID, err := services.SendInvoiceReadyEmail(account, client, project, invoice.BillingoInvoiceNumber, pdfBytes)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to send e-mail: " + err.Error()})
	}
	if err := database.GetDB().Create(&models.InvoiceReadyEmail{
		InvoiceID:      invoice.ID,
		ProjectID:      uint(projectID),
		ClientID:       client.ID,
		SentBy:         currentUserID,
		SentAt:         time.Now(),
		GmailMessageID: gmailMessageID,
	}).Error; err != nil {
		log.Printf("⚠️ Failed to record invoice-ready e-mail for invoice %d: %v", invoice.ID, err)
	}

	return c.JSON(fiber.Map{"success": true, "message": "E-mail elküldve"})
}

// RefreshPaymentStatuses - POST /api/v1/projects/:id/invoices/refresh-payment-status
// Re-checks Billingo's payment_status/paid_date for every successfully
// created invoice of this project and persists any change, so the invoice
// history can show which invoices the client has actually paid. This is a
// manual, user-triggered read-only Billingo call — there is no webhook
// configured, so nothing runs automatically in the background.
func (h *InvoiceHandler) RefreshPaymentStatuses(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var invoices []models.Invoice
	if err := database.GetDB().
		Where("project_id = ? AND status = 'created' AND billingo_invoice_id <> ''", projectID).
		Find(&invoices).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load invoices"})
	}

	failures := 0
	for i := range invoices {
		status, paidDate, err := h.billingoService.GetDocumentPaymentStatus(invoices[i].BillingoInvoiceID)
		if err != nil {
			log.Printf("⚠️ Failed to refresh payment status for invoice %d: %v", invoices[i].ID, err)
			failures++
			continue
		}
		if err := database.GetDB().Model(&invoices[i]).Updates(map[string]interface{}{
			"payment_status": status,
			"paid_date":      paidDate,
		}).Error; err != nil {
			log.Printf("⚠️ Failed to save payment status for invoice %d: %v", invoices[i].ID, err)
			failures++
		}
	}

	if failures > 0 && failures == len(invoices) {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Nem sikerült elérni a Billingót. Ellenőrizze a Billingo beállításokat."})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Fizetettség frissítve", "checked": len(invoices), "failed": failures})
}
