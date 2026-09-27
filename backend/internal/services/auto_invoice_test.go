// backend/internal/services/auto_invoice_test.go
package services

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestShouldSendFixedPriceCompletionNoticeWhenFullyConfigured(t *testing.T) {
	clientID := uint(5)
	project := models.Project{PricingType: "fixed", AutoInvoiceEnabled: true, AutoInvoiceClientID: &clientID}
	if !shouldSendFixedPriceCompletionNotice(project) {
		t.Fatal("expected a completion notice for a fully configured fixed-price project")
	}
}

func TestShouldNotSendFixedPriceCompletionNoticeForHourlyProject(t *testing.T) {
	clientID := uint(5)
	project := models.Project{PricingType: "hourly", AutoInvoiceEnabled: true, AutoInvoiceClientID: &clientID}
	if shouldSendFixedPriceCompletionNotice(project) {
		t.Fatal("expected no completion notice for an hourly project - that's the scheduler's job")
	}
}

func TestShouldNotSendFixedPriceCompletionNoticeWhenDisabled(t *testing.T) {
	clientID := uint(5)
	project := models.Project{PricingType: "fixed", AutoInvoiceEnabled: false, AutoInvoiceClientID: &clientID}
	if shouldSendFixedPriceCompletionNotice(project) {
		t.Fatal("expected no completion notice when auto-invoicing isn't enabled")
	}
}

func TestShouldNotSendFixedPriceCompletionNoticeWithoutClient(t *testing.T) {
	project := models.Project{PricingType: "fixed", AutoInvoiceEnabled: true, AutoInvoiceClientID: nil}
	if shouldSendFixedPriceCompletionNotice(project) {
		t.Fatal("expected no completion notice without a configured client")
	}
}
