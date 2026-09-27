// backend/internal/services/invoice_reminders_test.go
package services

import (
	"testing"
	"time"

	"dev-bridge-manager/internal/models"
)

func TestBuildInvoiceReminderText(t *testing.T) {
	subject, body := BuildInvoiceReminderText("Acme Kft.", "Website Redesign", "SZLA-2026-001", 5)

	wantSubject := "Fizetési emlékeztető - Website Redesign"
	if subject != wantSubject {
		t.Fatalf("expected subject %q, got %q", wantSubject, subject)
	}

	wantBody := "Kedves Acme Kft.!\n\nSzeretnénk emlékeztetni, hogy a(z) \"Website Redesign\" projekt SZLA-2026-001 számú számlája 5 napja lejárt, és még nem érkezett meg a kiegyenlítése.\n\nÜdvözlettel"
	if body != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}

func TestReminderIsDueWhenNoPriorReminderExists(t *testing.T) {
	if !reminderIsDue(nil, time.Now()) {
		t.Fatal("expected a reminder to be due when none has ever been created")
	}
}

func TestReminderIsNotDueWhilePendingApproval(t *testing.T) {
	latest := &models.InvoiceReminder{Status: "pending", CreatedAt: time.Now().AddDate(0, 0, -30)}
	if reminderIsDue(latest, time.Now()) {
		t.Fatal("expected no new reminder while one is still pending approval")
	}
}

func TestReminderIsNotDueBeforeRepeatIntervalElapses(t *testing.T) {
	latest := &models.InvoiceReminder{Status: "sent", CreatedAt: time.Now().AddDate(0, 0, -3)}
	if reminderIsDue(latest, time.Now()) {
		t.Fatal("expected no new reminder before the repeat interval has elapsed")
	}
}

func TestReminderIsDueOnceRepeatIntervalElapses(t *testing.T) {
	latest := &models.InvoiceReminder{Status: "sent", CreatedAt: time.Now().AddDate(0, 0, -7)}
	if !reminderIsDue(latest, time.Now()) {
		t.Fatal("expected a new reminder once the repeat interval has elapsed")
	}
}

func TestReminderIsDueAgainAfterDismissal(t *testing.T) {
	latest := &models.InvoiceReminder{Status: "dismissed", CreatedAt: time.Now().AddDate(0, 0, -7)}
	if !reminderIsDue(latest, time.Now()) {
		t.Fatal("expected a dismissed reminder to be reconsidered after the repeat interval")
	}
}
