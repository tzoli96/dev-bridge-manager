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
// after local validation already passed. Used only for pricing types that
// have no pre-existing reservation row (currently: hourly, which has no
// once-only rule). Fixed-price failures instead update the existing
// 'pending' reservation row in place — see markReservationFailed.
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

// isDuplicateKeyError reports whether a GORM/Postgres error is a duplicate
// key / unique constraint violation. Shared by the fixed-price reservation
// insert and the hourly final insert so the once-only-rule race maps
// consistently to a 409 in both places.
func isDuplicateKeyError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "unique constraint")
}

// markReservationCreated finalizes a fixed-price 'pending' reservation row
// into a 'created' row after a successful Billingo call, updating both the
// DB row and the in-memory struct so the handler's JSON response reflects
// the final state.
func (h *InvoiceHandler) markReservationCreated(invoice *models.Invoice, billingoInvoiceID, billingoInvoiceNumber string) error {
	invoice.Status = "created"
	invoice.BillingoInvoiceID = billingoInvoiceID
	invoice.BillingoInvoiceNumber = billingoInvoiceNumber
	return database.GetDB().Model(invoice).Updates(map[string]interface{}{
		"status":                  invoice.Status,
		"billingo_invoice_id":     invoice.BillingoInvoiceID,
		"billingo_invoice_number": invoice.BillingoInvoiceNumber,
	}).Error
}

// markReservationFailed turns a fixed-price 'pending' reservation row into a
// 'failed' row after a Billingo call error, instead of inserting a new row
// (which would violate the once-only unique index while the reservation
// still exists).
func (h *InvoiceHandler) markReservationFailed(invoice *models.Invoice, errMsg string) {
	invoice.Status = "failed"
	invoice.ErrorMessage = errMsg
	database.GetDB().Model(invoice).Updates(map[string]interface{}{
		"status":        invoice.Status,
		"error_message": invoice.ErrorMessage,
	})
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

	// reservation holds the fixed-price 'pending' row that reserves the
	// once-only right for this project before Billingo is ever called. It
	// stays nil for pricing types (currently only hourly) that have no
	// once-only rule and therefore no reservation row.
	var reservation *models.Invoice

	switch project.PricingType {
	case "fixed":
		if project.FixedPrice == nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no fixed price configured"})
		}

		amount = services.CalculateFixedAmount(*project.FixedPrice)
		description = fmt.Sprintf("%s - fixed price", project.Name)

		// Enforcement of the once-only rule now happens here, via this
		// INSERT racing against the partial unique index
		// idx_invoices_fixed_price_once (status IN ('created','pending')),
		// BEFORE any Billingo API call is made. This closes the TOCTOU
		// window that a pure pre-check SELECT (the old approach) could not:
		// two concurrent requests can no longer both reach the Billingo
		// call for the same project.
		reservation = &models.Invoice{
			ProjectID:   uint(projectID),
			ClientID:    req.ClientID,
			PricingType: "fixed",
			Amount:      amount,
			Status:      "pending",
			CreatedBy:   currentUserID,
		}
		if err := database.GetDB().Create(reservation).Error; err != nil {
			if isDuplicateKeyError(err) {
				return c.Status(409).JSON(models.InvoiceListResponse{Success: false, Message: "Ez a projekt már ki lett számlázva"})
			}
			return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Failed to reserve invoice slot"})
		}

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
		if reservation != nil {
			h.markReservationFailed(reservation, err.Error())
		} else {
			h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, err.Error())
		}
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: "Billingo error: " + err.Error()})
	}

	billingoInvoiceID, billingoInvoiceNumber, err := h.billingoService.CreateInvoice(partnerID, amount, description)
	if err != nil {
		if reservation != nil {
			h.markReservationFailed(reservation, err.Error())
		} else {
			h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, err.Error())
		}
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: "Billingo error: " + err.Error()})
	}

	var invoice models.Invoice
	if reservation != nil {
		// Fixed-price: finalize the existing 'pending' reservation row into
		// 'created' rather than inserting a new row.
		if err := h.markReservationCreated(reservation, billingoInvoiceID, billingoInvoiceNumber); err != nil {
			return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Invoice created in Billingo but failed to save locally"})
		}
		invoice = *reservation
	} else {
		// Hourly: no reservation row exists (no once-only rule), so insert
		// the 'created' row directly, unchanged from before.
		invoice = models.Invoice{
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
			if isDuplicateKeyError(err) {
				return c.Status(409).JSON(models.InvoiceListResponse{Success: false, Message: "Ez a projekt már ki lett számlázva"})
			}
			return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Invoice created in Billingo but failed to save locally"})
		}
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
