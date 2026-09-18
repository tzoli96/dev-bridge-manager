// backend/internal/handlers/invoice_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"fmt"
	"log"
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
	if err := database.GetDB().Model(invoice).Updates(map[string]interface{}{
		"status":        invoice.Status,
		"error_message": invoice.ErrorMessage,
	}).Error; err != nil {
		// A failed UPDATE here leaves the reservation row stuck at
		// 'pending', which permanently blocks all future invoicing for
		// this project under the once-only unique index — there is no
		// other recovery path, so this must be visible in the logs.
		log.Printf("⚠️ Failed to mark invoice reservation %d as failed (project %d): %v", invoice.ID, invoice.ProjectID, err)
	}
}

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

	var dueDate *time.Time
	if req.DueDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid due_date (expected YYYY-MM-DD)"})
		}
		dueDate = &parsed
	}

	var amount float64
	var periodStart, periodEnd *time.Time
	var description string
	// baseQuantity/baseUnit/baseUnitPrice describe the pricing-type-derived
	// base line item; hourly billing sets these to the period's total hours
	// / "óra" / per-hour rate so the invoice reads as hours x rate instead
	// of a single opaque quantity-1 line.
	baseQuantity := 1.0
	var baseUnit string
	var baseUnitPrice float64

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

		fixedPrice := *project.FixedPrice
		if req.BaseUnitPrice > 0 {
			fixedPrice = req.BaseUnitPrice
		}
		amount = services.CalculateFixedAmount(fixedPrice)
		baseUnit = client.BillingoUnit
		baseUnitPrice = amount
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

		hourlyRate := *project.HourlyRate
		if req.BaseUnitPrice > 0 {
			hourlyRate = req.BaseUnitPrice
		}
		amount = services.CalculateHourlyAmount(totalHours, hourlyRate)
		baseQuantity = totalHours
		baseUnit = "óra"
		baseUnitPrice = hourlyRate
		periodStart, periodEnd = &start, &end
		description = fmt.Sprintf("%s - %s to %s", project.Name, req.PeriodStart, req.PeriodEnd)

	default:
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Unsupported pricing type"})
	}

	if req.ItemName != "" {
		description = req.ItemName
	}

	// lines starts with the pricing-type-derived base item and is extended
	// with any user-supplied extra items (e.g. one-time fees, expenses).
	// Every line here becomes both a Billingo invoice item and a persisted
	// invoice_items row, so an invoice's full item breakdown survives beyond
	// this request.
	type invoiceLine struct {
		Name      string
		Quantity  float64
		Unit      string
		UnitPrice float64
		IsBase    bool
	}
	lines := []invoiceLine{
		{Name: description, Quantity: baseQuantity, Unit: baseUnit, UnitPrice: baseUnitPrice, IsBase: true},
	}
	for _, extra := range req.ExtraItems {
		name := strings.TrimSpace(extra.Name)
		if name == "" && extra.Quantity == 0 && extra.UnitPrice == 0 {
			continue // ignore blank placeholder rows
		}
		if name == "" {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Extra item name is required"})
		}
		if extra.Quantity <= 0 {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Extra item quantity must be greater than 0"})
		}
		lines = append(lines, invoiceLine{Name: name, Quantity: extra.Quantity, Unit: extra.Unit, UnitPrice: extra.UnitPrice})
		amount += extra.Quantity * extra.UnitPrice
	}

	if reservation != nil {
		reservation.ItemName = description
		reservation.DueDate = dueDate
		reservation.Amount = amount
		if err := database.GetDB().Model(reservation).Updates(map[string]interface{}{
			"item_name": reservation.ItemName,
			"due_date":  reservation.DueDate,
			"amount":    reservation.Amount,
		}).Error; err != nil {
			h.markReservationFailed(reservation, "failed to save item name/due date")
			return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Failed to reserve invoice slot"})
		}
	}

	billingoItems := make([]services.InvoiceLineItem, 0, len(lines))
	for _, l := range lines {
		billingoItems = append(billingoItems, services.InvoiceLineItem{
			Name: l.Name, Quantity: l.Quantity, Unit: l.Unit, UnitPrice: l.UnitPrice, UnitPriceType: client.BillingoUnitPriceType,
		})
	}

	partnerID, err := h.billingoService.EnsurePartner(&client)
	if err != nil {
		friendly := services.FriendlyBillingoError(err)
		if reservation != nil {
			h.markReservationFailed(reservation, friendly)
		} else {
			h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, friendly)
		}
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: friendly})
	}

	billingoInvoiceID, billingoInvoiceNumber, err := h.billingoService.CreateInvoice(partnerID, billingoItems, req.DueDate)
	if err != nil {
		friendly := services.FriendlyBillingoError(err)
		if reservation != nil {
			h.markReservationFailed(reservation, friendly)
		} else {
			h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, friendly)
		}
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: friendly})
	}

	var invoice models.Invoice
	if reservation != nil {
		// Fixed-price: finalize the existing 'pending' reservation row into
		// 'created' rather than inserting a new row.
		if err := h.markReservationCreated(reservation, billingoInvoiceID, billingoInvoiceNumber); err != nil {
			// The reservation row stays stuck at 'pending', permanently
			// blocking future invoicing for this project under the
			// once-only unique index — there is no other recovery path,
			// so this must be visible in the logs even though the client
			// already gets a 500.
			log.Printf("⚠️ Failed to finalize invoice reservation %d as created (project %d): %v", reservation.ID, reservation.ProjectID, err)
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
			ItemName:              description,
			DueDate:               dueDate,
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

	itemRows := make([]models.InvoiceItem, 0, len(lines))
	for _, l := range lines {
		itemRows = append(itemRows, models.InvoiceItem{
			InvoiceID:     invoice.ID,
			Name:          l.Name,
			Quantity:      l.Quantity,
			Unit:          l.Unit,
			UnitPrice:     l.UnitPrice,
			UnitPriceType: client.BillingoUnitPriceType,
			LineTotal:     l.Quantity * l.UnitPrice,
			IsBase:        l.IsBase,
		})
	}
	if err := database.GetDB().Create(&itemRows).Error; err != nil {
		// The invoice itself was already created (locally and in Billingo);
		// losing its item breakdown here is a display-only regression, not
		// a billing-correctness issue, so this only needs to be logged.
		log.Printf("⚠️ Failed to save invoice_items for invoice %d: %v", invoice.ID, err)
		itemRows = nil
	}

	response := toInvoiceResponse(invoice, client.Name, "")
	response.Items = itemRows

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
