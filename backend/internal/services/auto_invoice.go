// backend/internal/services/auto_invoice.go
package services

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"fmt"
	"log"
	"time"
)

// RunAutoInvoicing bills the previous calendar month, for every hourly
// project with auto-invoicing enabled, to its configured client — the same
// steps InvoiceHandler.CreateInvoice performs for a manual hourly invoice
// (sum logged hours, ensure the Billingo partner, create the Billingo
// invoice, persist the invoice + its base line item).
//
// It is safe to call more than once on the same day (see StartAutoInvoiceScheduler):
// each project is skipped if an invoice already exists for that exact
// project/period, so re-running never double-bills.
func RunAutoInvoicing() {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-24 * time.Hour)

	var projects []models.Project
	if err := database.GetDB().
		Where("auto_invoice_enabled = ? AND pricing_type = ? AND auto_invoice_client_id IS NOT NULL", true, "hourly").
		Find(&projects).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to load eligible projects: %v", err)
		return
	}

	for _, project := range projects {
		autoInvoiceProject(project, periodStart, periodEnd)
	}
}

func autoInvoiceProject(project models.Project, periodStart, periodEnd time.Time) {
	db := database.GetDB()

	var existing models.Invoice
	err := db.Where(
		"project_id = ? AND period_start = ? AND period_end = ? AND status IN ('created','pending')",
		project.ID, periodStart, periodEnd,
	).First(&existing).Error
	if err == nil {
		return // already invoiced (or being invoiced) for this period
	}

	if project.HourlyRate == nil {
		log.Printf("⚠️ Auto-invoicing: project %d has no hourly rate configured, skipping", project.ID)
		return
	}

	var client models.Client
	if err := db.First(&client, *project.AutoInvoiceClientID).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: project %d's configured client %d not found, skipping", project.ID, *project.AutoInvoiceClientID)
		return
	}

	totalHours, err := SumLoggedHours(project.ID, periodStart, periodEnd)
	if err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to sum hours for project %d: %v", project.ID, err)
		return
	}
	if totalHours <= 0 {
		log.Printf("ℹ️ Auto-invoicing: project %d has no logged hours for %s, skipping", project.ID, periodStart.Format("2006-01"))
		return
	}

	amount := CalculateHourlyAmount(totalHours, *project.HourlyRate)
	itemName := fmt.Sprintf("%s - %s to %s", project.Name, periodStart.Format("2006-01-02"), periodEnd.Format("2006-01-02"))

	billingoService := NewBillingoService()
	partnerID, err := billingoService.EnsurePartner(&client)
	if err != nil {
		recordFailedAutoInvoice(project, client.ID, periodStart, periodEnd, amount, FriendlyBillingoError(err))
		return
	}

	items := []InvoiceLineItem{{
		Name: itemName, Quantity: totalHours, Unit: "óra", UnitPrice: *project.HourlyRate, UnitPriceType: client.BillingoUnitPriceType,
	}}
	billingoInvoiceID, billingoInvoiceNumber, err := billingoService.CreateInvoice(partnerID, items, "")
	if err != nil {
		recordFailedAutoInvoice(project, client.ID, periodStart, periodEnd, amount, FriendlyBillingoError(err))
		return
	}

	invoice := models.Invoice{
		ProjectID:             project.ID,
		ClientID:              client.ID,
		BillingoInvoiceID:     billingoInvoiceID,
		BillingoInvoiceNumber: billingoInvoiceNumber,
		PricingType:           "hourly",
		PeriodStart:           &periodStart,
		PeriodEnd:             &periodEnd,
		ItemName:              itemName,
		Amount:                amount,
		Status:                "created",
		CreatedBy:             project.CreatedBy,
	}
	if err := db.Create(&invoice).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: invoice created in Billingo but failed to save locally for project %d: %v", project.ID, err)
		return
	}

	if err := db.Create(&models.InvoiceItem{
		InvoiceID:     invoice.ID,
		Name:          itemName,
		Quantity:      totalHours,
		Unit:          "óra",
		UnitPrice:     *project.HourlyRate,
		UnitPriceType: client.BillingoUnitPriceType,
		LineTotal:     amount,
		IsBase:        true,
	}).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to save invoice_items for invoice %d: %v", invoice.ID, err)
	}

	log.Printf("✅ Auto-invoicing: created invoice %d for project %d (%.2f óra, %.2f HUF)", invoice.ID, project.ID, totalHours, amount)
}

// recordFailedAutoInvoice persists an audit row for an auto-invoicing
// Billingo call that failed, mirroring InvoiceHandler.recordFailedInvoice so
// the failure shows up in the project's invoice history like a manually
// created failed invoice would.
func recordFailedAutoInvoice(project models.Project, clientID uint, periodStart, periodEnd time.Time, amount float64, errMsg string) {
	invoice := models.Invoice{
		ProjectID:    project.ID,
		ClientID:     clientID,
		PricingType:  "hourly",
		PeriodStart:  &periodStart,
		PeriodEnd:    &periodEnd,
		Amount:       amount,
		Status:       "failed",
		ErrorMessage: errMsg,
		CreatedBy:    project.CreatedBy,
	}
	database.GetDB().Create(&invoice)
}
