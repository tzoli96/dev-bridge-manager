// backend/internal/handlers/invoice_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"fmt"
	"strconv"
	"strings"
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

// checkInvoiceAccess - jogosultság ellenőrzése egy adott invoices.* jogra, admin/super_admin/manager fallback-kel
func (h *InvoiceHandler) checkInvoiceAccess(userID uint, action string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, action)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
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
		Amount:                inv.Amount,
		Status:                inv.Status,
		ErrorMessage:          inv.ErrorMessage,
		CreatedBy:             inv.CreatedBy,
		CreatedByName:         createdByName,
		CreatedAt:             inv.CreatedAt,
	}
}

func ptrInvoiceResponse(r models.InvoiceResponse) *models.InvoiceResponse { return &r }

// recordFailedInvoice persists an audit row for a Billingo call that failed
// after local validation already passed. A failed row never blocks a
// subsequent retry — only a status='created' row counts against the
// fixed-price once-only rule.
func (h *InvoiceHandler) recordFailedInvoice(projectID, clientID, createdBy uint, pricingType string, periodStart, periodEnd *time.Time, amount float64, errMsg string) {
	invoice := models.Invoice{
		ProjectID:    projectID,
		ClientID:     clientID,
		PricingType:  pricingType,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		Amount:       amount,
		Status:       "failed",
		ErrorMessage: errMsg,
		CreatedBy:    createdBy,
	}
	database.GetDB().Create(&invoice)
}

// CreateInvoice - POST /api/v1/projects/:id/invoices
func (h *InvoiceHandler) CreateInvoice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkInvoiceAccess(currentUserID, "invoices.create"); err != nil {
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

	var amount float64
	var periodStart, periodEnd *time.Time
	var description string

	switch project.PricingType {
	case "fixed":
		if project.FixedPrice == nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no fixed price configured"})
		}

		var existing models.Invoice
		if err := database.GetDB().Where("project_id = ? AND status = ?", projectID, "created").First(&existing).Error; err == nil {
			return c.Status(409).JSON(models.InvoiceListResponse{Success: false, Message: "Ez a projekt már ki lett számlázva"})
		}

		amount = services.CalculateFixedAmount(*project.FixedPrice)
		description = fmt.Sprintf("%s - fixed price", project.Name)

	case "hourly":
		if project.HourlyRate == nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no hourly rate configured"})
		}
		if req.PeriodStart == "" || req.PeriodEnd == "" {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "period_start and period_end are required for hourly projects"})
		}

		start, err := time.Parse("2006-01-02", req.PeriodStart)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid period_start (expected YYYY-MM-DD)"})
		}
		end, err := time.Parse("2006-01-02", req.PeriodEnd)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid period_end (expected YYYY-MM-DD)"})
		}
		if start.After(end) {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "period_start must not be after period_end"})
		}

		totalHours, err := services.SumLoggedHours(uint(projectID), start, end)
		if err != nil {
			return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Error summing logged hours"})
		}

		amount = services.CalculateHourlyAmount(totalHours, *project.HourlyRate)
		periodStart, periodEnd = &start, &end
		description = fmt.Sprintf("%s - %s to %s", project.Name, req.PeriodStart, req.PeriodEnd)

	default:
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Unsupported pricing type"})
	}

	partnerID, err := h.billingoService.EnsurePartner(&client)
	if err != nil {
		h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, err.Error())
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: "Billingo error: " + err.Error()})
	}

	billingoInvoiceID, billingoInvoiceNumber, err := h.billingoService.CreateInvoice(partnerID, amount, description)
	if err != nil {
		h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, err.Error())
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: "Billingo error: " + err.Error()})
	}

	invoice := models.Invoice{
		ProjectID:             uint(projectID),
		ClientID:              req.ClientID,
		BillingoInvoiceID:     billingoInvoiceID,
		BillingoInvoiceNumber: billingoInvoiceNumber,
		PricingType:           project.PricingType,
		PeriodStart:           periodStart,
		PeriodEnd:             periodEnd,
		Amount:                amount,
		Status:                "created",
		CreatedBy:             currentUserID,
	}

	if err := database.GetDB().Create(&invoice).Error; err != nil {
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "unique constraint") {
			return c.Status(409).JSON(models.InvoiceListResponse{Success: false, Message: "Ez a projekt már ki lett számlázva"})
		}
		return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Invoice created in Billingo but failed to save locally"})
	}

	return c.Status(201).JSON(models.InvoiceListResponse{
		Success: true,
		Message: "Invoice created successfully",
		Invoice: ptrInvoiceResponse(toInvoiceResponse(invoice, client.Name, "")),
	})
}

// GetProjectInvoices - GET /api/v1/projects/:id/invoices
func (h *InvoiceHandler) GetProjectInvoices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkInvoiceAccess(currentUserID, "invoices.read"); err != nil {
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

	response := make([]models.InvoiceResponse, 0, len(rows))
	for _, row := range rows {
		response = append(response, toInvoiceResponse(row.Invoice, row.ClientName, row.CreatedByName))
	}

	return c.JSON(models.InvoiceListResponse{
		Success:  true,
		Message:  "Invoices retrieved successfully",
		Invoices: response,
		Count:    len(response),
	})
}
