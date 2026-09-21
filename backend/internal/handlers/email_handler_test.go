// backend/internal/handlers/email_handler_test.go
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIsValidEmailCategory(t *testing.T) {
	valid := []string{
		models.EmailCategoryClient,
		models.EmailCategoryBilling,
		models.EmailCategoryMarketing,
		models.EmailCategorySystem,
		models.EmailCategoryOther,
	}
	for _, c := range valid {
		if !isValidEmailCategory(c) {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []string{"", "bogus", "UGYFEL"}
	for _, c := range invalid {
		if isValidEmailCategory(c) {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}
