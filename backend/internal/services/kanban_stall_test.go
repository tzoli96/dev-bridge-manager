// backend/internal/services/kanban_stall_test.go
package services

import (
	"testing"
	"time"

	"dev-bridge-manager/internal/models"
)

func TestTaskIsStalledOnceThresholdElapses(t *testing.T) {
	enteredAt := time.Now().Add(-5 * 24 * time.Hour)
	if !taskIsStalled(enteredAt, time.Now()) {
		t.Fatal("expected a task sitting 5 days in the same column to be stalled")
	}
}

func TestTaskIsNotStalledBeforeThresholdElapses(t *testing.T) {
	enteredAt := time.Now().Add(-4 * 24 * time.Hour)
	if taskIsStalled(enteredAt, time.Now()) {
		t.Fatal("expected a task sitting only 4 days in the same column to not be stalled yet")
	}
}

func TestStallFlagIsDueWhenNoPriorFlagExists(t *testing.T) {
	if !stallFlagIsDue(nil, time.Now()) {
		t.Fatal("expected a flag to be due when none exists yet")
	}
}

func TestStallFlagIsNotDueWhilePending(t *testing.T) {
	latest := &models.KanbanStallFlag{Status: "pending", CreatedAt: time.Now()}
	if stallFlagIsDue(latest, time.Now()) {
		t.Fatal("expected no new flag while one is already pending")
	}
}

func TestStallFlagIsNotDueBeforeRepeatIntervalElapses(t *testing.T) {
	latest := &models.KanbanStallFlag{Status: "dismissed", CreatedAt: time.Now().Add(-4 * 24 * time.Hour)}
	if stallFlagIsDue(latest, time.Now()) {
		t.Fatal("expected no new flag before the repeat interval elapses")
	}
}

func TestStallFlagIsDueOnceRepeatIntervalElapses(t *testing.T) {
	latest := &models.KanbanStallFlag{Status: "dismissed", CreatedAt: time.Now().Add(-5 * 24 * time.Hour)}
	if !stallFlagIsDue(latest, time.Now()) {
		t.Fatal("expected a new flag once the repeat interval has elapsed since dismissal")
	}
}
