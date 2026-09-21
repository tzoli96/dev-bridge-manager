// backend/internal/services/invoice_notice_test.go
package services

import (
	"testing"
	"time"
)

func TestBuildInvoiceNoticeTextWithoutPeriod(t *testing.T) {
	subject, body := BuildInvoiceNoticeText("Acme Kft.", "Website Redesign", nil, nil, "")

	wantSubject := "Számla értesítő - Website Redesign"
	if subject != wantSubject {
		t.Fatalf("expected subject %q, got %q", wantSubject, subject)
	}

	wantBody := "Kedves Acme Kft.!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"Website Redesign\" projekt kapcsán.\n\nÜdvözlettel"
	if body != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}

func TestBuildInvoiceNoticeTextWithPeriod(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

	_, body := BuildInvoiceNoticeText("Acme Kft.", "Website Redesign", &start, &end, "")

	wantBody := "Kedves Acme Kft.!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"Website Redesign\" projekt kapcsán a 2026.08.01 - 2026.08.31 időszakra vonatkozóan.\n\nÜdvözlettel"
	if body != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}

func TestBuildInvoiceNoticeTextWithDraftSummary(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

	_, body := BuildInvoiceNoticeText("Acme Kft.", "Website Redesign", &start, &end, "Projekt: Website Redesign\nÖsszesen: 12 óra × 20 000 Ft = 240 000 Ft")

	wantBody := "Kedves Acme Kft.!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"Website Redesign\" projekt kapcsán a 2026.08.01 - 2026.08.31 időszakra vonatkozóan.\n\nÜdvözlettel\n\nProjekt: Website Redesign\nÖsszesen: 12 óra × 20 000 Ft = 240 000 Ft"
	if body != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}
