// backend/internal/services/invoice_reconciliation_test.go
package services

import (
	"testing"
	"time"

	"dev-bridge-manager/internal/models"
)

func TestHoursDiscrepancyDetectsDrift(t *testing.T) {
	if !hoursDiscrepancy(10, 12) {
		t.Fatal("expected a 2-hour drift to be detected as a discrepancy")
	}
}

func TestHoursDiscrepancyIgnoresFloatingPointNoise(t *testing.T) {
	if hoursDiscrepancy(10, 10.001) {
		t.Fatal("expected a negligible difference to not count as a discrepancy")
	}
}

func TestStaleHoursIsDueOnceThresholdElapses(t *testing.T) {
	oldest := time.Now().Add(-46 * 24 * time.Hour)
	if !staleHoursIsDue(oldest, time.Now()) {
		t.Fatal("expected hours older than the threshold to be due")
	}
}

func TestStaleHoursIsNotDueBeforeThresholdElapses(t *testing.T) {
	oldest := time.Now().Add(-10 * 24 * time.Hour)
	if staleHoursIsDue(oldest, time.Now()) {
		t.Fatal("expected recently-logged hours to not be due yet")
	}
}

func TestReconciliationFlagIsDueWhenNoPriorFlagExists(t *testing.T) {
	if !reconciliationFlagIsDue(nil, time.Now()) {
		t.Fatal("expected a flag to be due when none exists yet")
	}
}

func TestReconciliationFlagIsNotDueWhilePending(t *testing.T) {
	latest := &models.InvoiceReconciliationFlag{Status: "pending", CreatedAt: time.Now()}
	if reconciliationFlagIsDue(latest, time.Now()) {
		t.Fatal("expected no new flag while one is already pending")
	}
}

func TestReconciliationFlagIsNotDueBeforeRepeatIntervalElapses(t *testing.T) {
	latest := &models.InvoiceReconciliationFlag{Status: "dismissed", CreatedAt: time.Now().Add(-4 * 24 * time.Hour)}
	if reconciliationFlagIsDue(latest, time.Now()) {
		t.Fatal("expected no new flag before the repeat interval elapses")
	}
}

func TestReconciliationFlagIsDueOnceRepeatIntervalElapses(t *testing.T) {
	latest := &models.InvoiceReconciliationFlag{Status: "dismissed", CreatedAt: time.Now().Add(-8 * 24 * time.Hour)}
	if !reconciliationFlagIsDue(latest, time.Now()) {
		t.Fatal("expected a new flag once the repeat interval elapses")
	}
}
