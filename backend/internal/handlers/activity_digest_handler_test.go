// backend/internal/handlers/activity_digest_handler_test.go
package handlers

import (
	"testing"
	"time"
)

func TestDigestDateRangeDefaultsToYesterday(t *testing.T) {
	now := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)
	start, end, err := digestDateRange(now, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStart := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("got [%v, %v), want [%v, %v)", start, end, wantStart, wantEnd)
	}
}

func TestDigestDateRangeWithExplicitDate(t *testing.T) {
	now := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)
	start, end, err := digestDateRange(now, "2026-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStart := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("got [%v, %v), want [%v, %v)", start, end, wantStart, wantEnd)
	}
}

func TestDigestDateRangeRejectsInvalidDate(t *testing.T) {
	if _, _, err := digestDateRange(time.Now(), "not-a-date"); err == nil {
		t.Fatal("expected an error for an invalid date param, got nil")
	}
}
