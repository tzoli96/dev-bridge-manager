// backend/internal/services/project_renewal_test.go
package services

import (
	"testing"
	"time"

	"dev-bridge-manager/internal/models"
)

func TestRenewalIsDueWithinLeadTime(t *testing.T) {
	contractEndDate := time.Now().Add(29 * 24 * time.Hour)
	if !renewalIsDue(contractEndDate, time.Now()) {
		t.Fatal("expected a contract ending in 29 days to be due for a renewal flag")
	}
}

func TestRenewalIsDueOnceAlreadyPast(t *testing.T) {
	contractEndDate := time.Now().Add(-24 * time.Hour)
	if !renewalIsDue(contractEndDate, time.Now()) {
		t.Fatal("expected an already-past contract end date to be due for a renewal flag")
	}
}

func TestRenewalIsNotDueBeforeLeadTime(t *testing.T) {
	contractEndDate := time.Now().Add(31 * 24 * time.Hour)
	if renewalIsDue(contractEndDate, time.Now()) {
		t.Fatal("expected a contract ending in 31 days to not be due yet")
	}
}

func TestRenewalFlagIsDueWhenNoPriorFlagExists(t *testing.T) {
	if !renewalFlagIsDue(nil, time.Now()) {
		t.Fatal("expected a flag to be due when none exists yet")
	}
}

func TestRenewalFlagIsNotDueWhilePending(t *testing.T) {
	latest := &models.ProjectRenewalFlag{Status: "pending", CreatedAt: time.Now()}
	if renewalFlagIsDue(latest, time.Now()) {
		t.Fatal("expected no new flag while one is already pending")
	}
}

func TestRenewalFlagIsNotDueBeforeRepeatIntervalElapses(t *testing.T) {
	latest := &models.ProjectRenewalFlag{Status: "dismissed", CreatedAt: time.Now().Add(-6 * 24 * time.Hour)}
	if renewalFlagIsDue(latest, time.Now()) {
		t.Fatal("expected no new flag before the repeat interval elapses")
	}
}

func TestRenewalFlagIsDueOnceRepeatIntervalElapses(t *testing.T) {
	latest := &models.ProjectRenewalFlag{Status: "dismissed", CreatedAt: time.Now().Add(-7 * 24 * time.Hour)}
	if !renewalFlagIsDue(latest, time.Now()) {
		t.Fatal("expected a new flag once the repeat interval has elapsed since dismissal")
	}
}
