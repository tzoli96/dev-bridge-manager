// backend/internal/services/client_status_summary_test.go
package services

import (
	"testing"
	"time"
)

func TestWeekBeforeMondayReturnsPriorMondayToSunday(t *testing.T) {
	// 2026-09-28 is a Monday.
	now := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	start, end := weekBeforeMonday(now)
	wantStart := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("expected start %v, got %v", wantStart, start)
	}
	if !end.Equal(wantEnd) {
		t.Fatalf("expected end %v, got %v", wantEnd, end)
	}
}

func TestWeekBeforeMondayFromMidWeekUsesMostRecentMonday(t *testing.T) {
	// 2026-09-30 is a Wednesday; the most recent Monday is 2026-09-28, so the
	// prior completed week is 2026-09-21 - 2026-09-27, same as if called on
	// that Monday itself.
	now := time.Date(2026, 9, 30, 15, 30, 0, 0, time.UTC)
	start, end := weekBeforeMonday(now)
	wantStart := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("expected start %v, got %v", wantStart, start)
	}
	if !end.Equal(wantEnd) {
		t.Fatalf("expected end %v, got %v", wantEnd, end)
	}
}

func TestClientWeeklySummaryHasActivityWithCompletedTasks(t *testing.T) {
	s := ClientWeeklySummary{CompletedTasks: []string{"Task A"}}
	if !s.HasActivity() {
		t.Fatal("expected HasActivity to be true when there are completed tasks")
	}
}

func TestClientWeeklySummaryHasActivityWithHoursLogged(t *testing.T) {
	s := ClientWeeklySummary{HoursLogged: 3.5}
	if !s.HasActivity() {
		t.Fatal("expected HasActivity to be true when hours were logged")
	}
}

func TestClientWeeklySummaryHasActivityWithInvoiceActivity(t *testing.T) {
	if !(ClientWeeklySummary{InvoicesCreated: 1}).HasActivity() {
		t.Fatal("expected HasActivity to be true when an invoice was created")
	}
	if !(ClientWeeklySummary{InvoicesPaid: 1}).HasActivity() {
		t.Fatal("expected HasActivity to be true when an invoice was paid")
	}
}

func TestClientWeeklySummaryHasActivityFalseWhenAllZero(t *testing.T) {
	s := ClientWeeklySummary{}
	if s.HasActivity() {
		t.Fatal("expected HasActivity to be false with no signals")
	}
}
