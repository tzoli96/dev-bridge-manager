// backend/internal/services/invoice_creation.go
package services

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"fmt"
	"log"
	"strings"
	"time"
)

// recordFailedInvoice persists an audit row for a Billingo call that failed
// after local validation already passed. Used only for pricing types that
// have no pre-existing reservation row (currently: hourly, which has no
// once-only rule). Fixed-price failures instead update the existing
// 'pending' reservation row in place — see markReservationFailed.
func recordFailedInvoice(projectID, clientID, createdBy uint, pricingType string, periodStart, periodEnd *time.Time, amount float64, errMsg string) {
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
func markReservationCreated(invoice *models.Invoice, billingoInvoiceID, billingoInvoiceNumber string) error {
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
func markReservationFailed(invoice *models.Invoice, errMsg string) {
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

// CreateInvoiceForProject runs the shared pricing/Billingo/persistence logic
// for creating an invoice. Used by the manual InvoiceHandler.CreateInvoice
// handler, InvoiceNoticeHandler.ApproveInvoiceNotice (via ApproveInvoiceNotice
// below), and the auto-invoice scheduler's auto-approve path, so none of the
// three paths can ever diverge in how an invoice is priced or persisted.
// Assumes the caller has already resolved project/client and validated
// req.ClientID != 0.
// httpStatus == 0 means success (invoice/items are set); otherwise
// httpStatus/message are ready to write straight into an HTTP response.
func CreateInvoiceForProject(billingoService *BillingoService, project models.Project, client models.Client, req models.InvoiceCreateRequest, createdBy uint) (invoice *models.Invoice, items []models.InvoiceItem, httpStatus int, message string) {
	projectID := project.ID

	var dueDate *time.Time
	if req.DueDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			return nil, nil, 400, "Invalid due_date (expected YYYY-MM-DD)"
		}
		dueDate = &parsed
	}

	var amount float64
	var periodStart, periodEnd *time.Time
	var description string
	baseQuantity := 1.0
	var baseUnit string
	var baseUnitPrice float64
	var reservation *models.Invoice

	switch project.PricingType {
	case "fixed":
		if project.FixedPrice == nil {
			return nil, nil, 400, "Project has no fixed price configured"
		}

		fixedPrice := *project.FixedPrice
		if req.BaseUnitPrice > 0 {
			fixedPrice = req.BaseUnitPrice
		}
		amount = CalculateFixedAmount(fixedPrice)
		baseUnit = client.BillingoUnit
		baseUnitPrice = amount
		description = fmt.Sprintf("%s - fixed price", project.Name)

		reservation = &models.Invoice{
			ProjectID:   projectID,
			ClientID:    req.ClientID,
			PricingType: "fixed",
			Amount:      amount,
			Status:      "pending",
			CreatedBy:   createdBy,
		}
		if err := database.GetDB().Create(reservation).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, nil, 409, "Ez a projekt már ki lett számlázva"
			}
			return nil, nil, 500, "Failed to reserve invoice slot"
		}

	case "hourly":
		if project.HourlyRate == nil {
			return nil, nil, 400, "Project has no hourly rate configured"
		}
		if req.PeriodStart == "" || req.PeriodEnd == "" {
			return nil, nil, 400, "period_start and period_end are required for hourly projects"
		}

		start, err := time.Parse("2006-01-02", req.PeriodStart)
		if err != nil {
			return nil, nil, 400, "Invalid period_start (expected YYYY-MM-DD)"
		}
		end, err := time.Parse("2006-01-02", req.PeriodEnd)
		if err != nil {
			return nil, nil, 400, "Invalid period_end (expected YYYY-MM-DD)"
		}
		if start.After(end) {
			return nil, nil, 400, "period_start must not be after period_end"
		}

		totalHours, err := SumLoggedHours(projectID, start, end)
		if err != nil {
			return nil, nil, 500, "Error summing logged hours"
		}

		hourlyRate := *project.HourlyRate
		if req.BaseUnitPrice > 0 {
			hourlyRate = req.BaseUnitPrice
		}
		amount = CalculateHourlyAmount(totalHours, hourlyRate)
		baseQuantity = totalHours
		baseUnit = "óra"
		baseUnitPrice = hourlyRate
		periodStart, periodEnd = &start, &end
		description = fmt.Sprintf("%s - %s to %s", project.Name, req.PeriodStart, req.PeriodEnd)

	default:
		return nil, nil, 400, "Unsupported pricing type"
	}

	if req.ItemName != "" {
		description = req.ItemName
	}

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
			continue
		}
		if name == "" {
			return nil, nil, 400, "Extra item name is required"
		}
		if extra.Quantity <= 0 {
			return nil, nil, 400, "Extra item quantity must be greater than 0"
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
			markReservationFailed(reservation, "failed to save item name/due date")
			return nil, nil, 500, "Failed to reserve invoice slot"
		}
	}

	billingoItems := make([]InvoiceLineItem, 0, len(lines))
	for _, l := range lines {
		billingoItems = append(billingoItems, InvoiceLineItem{
			Name: l.Name, Quantity: l.Quantity, Unit: l.Unit, UnitPrice: l.UnitPrice, UnitPriceType: client.BillingoUnitPriceType,
		})
	}

	partnerID, err := billingoService.EnsurePartner(&client)
	if err != nil {
		friendly := FriendlyBillingoError(err)
		if reservation != nil {
			markReservationFailed(reservation, friendly)
		} else {
			recordFailedInvoice(projectID, req.ClientID, createdBy, project.PricingType, periodStart, periodEnd, amount, friendly)
		}
		return nil, nil, 502, friendly
	}

	billingoInvoiceID, billingoInvoiceNumber, err := billingoService.CreateInvoice(partnerID, billingoItems, req.DueDate)
	if err != nil {
		friendly := FriendlyBillingoError(err)
		if reservation != nil {
			markReservationFailed(reservation, friendly)
		} else {
			recordFailedInvoice(projectID, req.ClientID, createdBy, project.PricingType, periodStart, periodEnd, amount, friendly)
		}
		return nil, nil, 502, friendly
	}

	var inv models.Invoice
	if reservation != nil {
		if err := markReservationCreated(reservation, billingoInvoiceID, billingoInvoiceNumber); err != nil {
			log.Printf("⚠️ Failed to finalize invoice reservation %d as created (project %d): %v", reservation.ID, reservation.ProjectID, err)
			return nil, nil, 500, "Invoice created in Billingo but failed to save locally"
		}
		inv = *reservation
	} else {
		inv = models.Invoice{
			ProjectID:             projectID,
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
			CreatedBy:             createdBy,
		}
		if err := database.GetDB().Create(&inv).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, nil, 409, "Ez a projekt már ki lett számlázva"
			}
			return nil, nil, 500, "Invoice created in Billingo but failed to save locally"
		}
	}

	itemRows := make([]models.InvoiceItem, 0, len(lines))
	for _, l := range lines {
		itemRows = append(itemRows, models.InvoiceItem{
			InvoiceID:     inv.ID,
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
		log.Printf("⚠️ Failed to save invoice_items for invoice %d: %v", inv.ID, err)
		itemRows = nil
	}

	return &inv, itemRows, 0, ""
}
