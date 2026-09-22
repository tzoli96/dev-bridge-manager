// backend/internal/handlers/email_handler_test.go
package handlers

import (
	"strings"
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

func TestGenerateSnippet(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", "", ""},
		{"short body left unchanged", "Szia, köszönöm az emailt.", "Szia, köszönöm az emailt."},
		{
			"long body cut at a word boundary around 200 chars",
			strings.Repeat("alma ", 60), // 300 chars, well past the 200 cutoff
			strings.TrimSpace(strings.Repeat("alma ", 60))[:199],
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateSnippet(tc.body)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if len(got) > 200 {
				t.Fatalf("snippet longer than 200 chars: %d", len(got))
			}
		})
	}
}
