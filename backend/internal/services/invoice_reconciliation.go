// backend/internal/services/invoice_reconciliation.go
package services

import (
	"fmt"
	"log"
	"math"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const invoiceReconciliationRepeatInterval = 7 * 24 * time.Hour
const invoiceReconciliationCheckInterval = 24 * time.Hour

// staleHoursThreshold is how old the oldest never-invoiced logged hour has
// to be before it's flagged - long enough that the current/just-closed
// month's normal pre-invoicing lag never trips it.
const staleHoursThreshold = 45 * 24 * time.Hour

const hoursEpsilon = 0.01

// hoursDiscrepancy reports whether currentHours has drifted from
// invoicedHours by more than floating-point noise - i.e. a time entry within
// the invoiced period was edited/added/deleted after the invoice was
// created.
func hoursDiscrepancy(invoicedHours, currentHours float64) bool {
	return math.Abs(currentHours-invoicedHours) > hoursEpsilon
}

// staleHoursIsDue reports whether the oldest never-invoiced logged hour is
// old enough to flag.
func staleHoursIsDue(oldestUninvoiced time.Time, now time.Time) bool {
	return now.Sub(oldestUninvoiced) >= staleHoursThreshold
}

// reconciliationFlagIsDue mirrors stallFlagIsDue/renewalFlagIsDue: no flag
// while one is already pending, otherwise re-flag once
// invoiceReconciliationRepeatInterval has elapsed since the last one.
func reconciliationFlagIsDue(latest *models.InvoiceReconciliationFlag, now time.Time) bool {
	if latest == nil {
		return true
	}
	if latest.Status == "pending" {
		return false
	}
	return now.Sub(latest.CreatedAt) >= invoiceReconciliationRepeatInterval
}

func RunInvoiceReconciliationCheck() {
	db := database.GetDB()
	now := time.Now()

	cleanupResolvedReconciliationFlags(db, now)
	checkInvoiceDiscrepancies(db, now)
	checkStaleUninvoicedHours(db, now)
}

// cleanupResolvedReconciliationFlags deletes pending flags whose underlying
// condition no longer holds, same as RunProjectRenewalCheck/
// RunKanbanStallCheck do for their own flag types.
func cleanupResolvedReconciliationFlags(db *gorm.DB, now time.Time) {
	var pending []models.InvoiceReconciliationFlag
	if err := db.Where("status = ?", "pending").Find(&pending).Error; err != nil {
		log.Printf("⚠️ Invoice reconciliation check: failed to load pending flags: %v", err)
		return
	}

	for _, flag := range pending {
		resolved := false

		switch flag.Type {
		case "discrepancy":
			if flag.InvoiceID == nil {
				resolved = true
				break
			}
			var invoice models.Invoice
			if err := db.First(&invoice, *flag.InvoiceID).Error; err != nil {
				resolved = true
				break
			}
			if invoice.InvoicedHours == nil || invoice.PeriodStart == nil || invoice.PeriodEnd == nil {
				resolved = true
				break
			}
			currentHours, err := SumLoggedHours(flag.ProjectID, *invoice.PeriodStart, *invoice.PeriodEnd)
			if err != nil {
				continue
			}
			resolved = !hoursDiscrepancy(*invoice.InvoicedHours, currentHours)

		case "stale_hours":
			oldest, hasAny, err := oldestUninvoicedEntryDate(db, flag.ProjectID)
			if err != nil {
				continue
			}
			resolved = !hasAny || !staleHoursIsDue(oldest, now)
		}

		if resolved {
			db.Delete(&models.InvoiceReconciliationFlag{}, flag.ID)
		}
	}
}

func checkInvoiceDiscrepancies(db *gorm.DB, now time.Time) {
	var invoices []models.Invoice
	err := db.Where("pricing_type = ? AND status = ? AND invoiced_hours IS NOT NULL AND period_start IS NOT NULL AND period_end IS NOT NULL",
		"hourly", "created").Find(&invoices).Error
	if err != nil {
		log.Printf("⚠️ Invoice reconciliation check: failed to load hourly invoices: %v", err)
		return
	}

	for _, invoice := range invoices {
		currentHours, err := SumLoggedHours(invoice.ProjectID, *invoice.PeriodStart, *invoice.PeriodEnd)
		if err != nil {
			log.Printf("⚠️ Invoice reconciliation check: failed to sum hours for invoice %d: %v", invoice.ID, err)
			continue
		}
		if !hoursDiscrepancy(*invoice.InvoicedHours, currentHours) {
			continue
		}

		var latest models.InvoiceReconciliationFlag
		latestErr := db.Where("invoice_id = ? AND type = ?", invoice.ID, "discrepancy").
			Order("created_at DESC").First(&latest).Error
		var latestPtr *models.InvoiceReconciliationFlag
		if latestErr == nil {
			latestPtr = &latest
		}
		if !reconciliationFlagIsDue(latestPtr, now) {
			continue
		}

		invoiceID := invoice.ID
		flag := models.InvoiceReconciliationFlag{
			ProjectID: invoice.ProjectID,
			InvoiceID: &invoiceID,
			Type:      "discrepancy",
			Details: fmt.Sprintf("A(z) #%d számla %.2f órára készült, jelenleg %.2f óra van naplózva ugyanerre az időszakra.",
				invoice.ID, *invoice.InvoicedHours, currentHours),
			Status:    "pending",
			CreatedAt: now,
		}
		if err := db.Create(&flag).Error; err != nil {
			log.Printf("⚠️ Invoice reconciliation check: failed to create discrepancy flag for invoice %d: %v", invoice.ID, err)
		}
	}
}

func checkStaleUninvoicedHours(db *gorm.DB, now time.Time) {
	var projects []models.Project
	if err := db.Where("pricing_type = ?", "hourly").Find(&projects).Error; err != nil {
		log.Printf("⚠️ Invoice reconciliation check: failed to load hourly projects: %v", err)
		return
	}

	for _, project := range projects {
		oldest, hasAny, err := oldestUninvoicedEntryDate(db, project.ID)
		if err != nil {
			log.Printf("⚠️ Invoice reconciliation check: failed to load uninvoiced hours for project %d: %v", project.ID, err)
			continue
		}
		if !hasAny || !staleHoursIsDue(oldest, now) {
			continue
		}

		var latest models.InvoiceReconciliationFlag
		latestErr := db.Where("project_id = ? AND type = ?", project.ID, "stale_hours").
			Order("created_at DESC").First(&latest).Error
		var latestPtr *models.InvoiceReconciliationFlag
		if latestErr == nil {
			latestPtr = &latest
		}
		if !reconciliationFlagIsDue(latestPtr, now) {
			continue
		}

		flag := models.InvoiceReconciliationFlag{
			ProjectID: project.ID,
			Type:      "stale_hours",
			Details: fmt.Sprintf("Vannak %s óta naplózott, még sosem számlázott órák ezen a projekten.",
				oldest.Format("2006.01.02")),
			Status:    "pending",
			CreatedAt: now,
		}
		if err := db.Create(&flag).Error; err != nil {
			log.Printf("⚠️ Invoice reconciliation check: failed to create stale-hours flag for project %d: %v", project.ID, err)
		}
	}
}

// oldestUninvoicedEntryDate returns the oldest task_time_entries.date for
// projectID that doesn't fall within any already-invoiced period (see
// InvoicedPeriodsByProject/IsDateInvoiced). hasAny is false if every logged
// hour has already been invoiced (or there are none).
func oldestUninvoicedEntryDate(db *gorm.DB, projectID uint) (oldest time.Time, hasAny bool, err error) {
	var dates []time.Time
	err = db.Table("task_time_entries").
		Select("task_time_entries.date").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("tasks.project_id = ?", projectID).
		Pluck("task_time_entries.date", &dates).Error
	if err != nil {
		return time.Time{}, false, err
	}
	if len(dates) == 0 {
		return time.Time{}, false, nil
	}

	periodsByProject, err := InvoicedPeriodsByProject([]uint{projectID})
	if err != nil {
		return time.Time{}, false, err
	}
	periods := periodsByProject[projectID]

	for _, date := range dates {
		if IsDateInvoiced(date, periods) {
			continue
		}
		if !hasAny || date.Before(oldest) {
			oldest = date
			hasAny = true
		}
	}
	return oldest, hasAny, nil
}

func StartInvoiceReconciliationScheduler() {
	log.Println("📊 Invoice reconciliation check: running")
	RunInvoiceReconciliationCheck()

	ticker := time.NewTicker(invoiceReconciliationCheckInterval)
	for range ticker.C {
		log.Println("📊 Invoice reconciliation check: running")
		RunInvoiceReconciliationCheck()
	}
}
