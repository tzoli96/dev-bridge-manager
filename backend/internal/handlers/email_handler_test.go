// backend/internal/handlers/email_handler_test.go
package handlers

import "testing"

func TestIsValidEmailCategory(t *testing.T) {
	valid := []string{"ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"}
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
