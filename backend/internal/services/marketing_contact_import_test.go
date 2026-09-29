// backend/internal/services/marketing_contact_import_test.go
package services

import (
	"strings"
	"testing"
)

func TestParseContactsCSVParsesValidRows(t *testing.T) {
	csv := "email,first_name,last_name,subscribed,tags,source,notes\n" +
		"jane@example.com,Jane,Doe,true,\"vip, newsletter\",import,VIP customer\n" +
		"john@example.com,John,Smith,false,,manual,\n"

	parsed, err := ParseContactsCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(parsed.Rows))
	}
	if len(parsed.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", parsed.Errors)
	}

	first := parsed.Rows[0]
	if first.Email != "jane@example.com" || first.FirstName != "Jane" || first.LastName != "Doe" {
		t.Fatalf("unexpected first row: %+v", first)
	}
	if !first.Subscribed {
		t.Fatal("expected first row to be subscribed")
	}
	if first.Tags != "vip, newsletter" {
		t.Fatalf("unexpected tags: %q", first.Tags)
	}

	second := parsed.Rows[1]
	if second.Subscribed {
		t.Fatal("expected second row to be unsubscribed")
	}
}

func TestParseContactsCSVMissingEmailColumnErrors(t *testing.T) {
	csv := "first_name,last_name\nJane,Doe\n"
	if _, err := ParseContactsCSV(strings.NewReader(csv)); err == nil {
		t.Fatal("expected an error for a CSV with no email column")
	}
}

func TestParseContactsCSVSkipsInvalidEmailRow(t *testing.T) {
	csv := "email,first_name\nnot-an-email,Jane\nvalid@example.com,John\n"
	parsed, err := ParseContactsCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.Rows) != 1 {
		t.Fatalf("expected 1 valid row, got %d", len(parsed.Rows))
	}
	if len(parsed.Errors) != 1 {
		t.Fatalf("expected 1 row error, got %v", parsed.Errors)
	}
	if parsed.Rows[0].Email != "valid@example.com" {
		t.Fatalf("unexpected surviving row: %+v", parsed.Rows[0])
	}
}

func TestParseContactsCSVSubscribedColumnParsing(t *testing.T) {
	csv := "email,subscribed\n" +
		"a@example.com,true\n" +
		"b@example.com,1\n" +
		"c@example.com,yes\n" +
		"d@example.com,false\n" +
		"e@example.com,\n"

	parsed, err := ParseContactsCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []bool{true, true, true, false, false}
	if len(parsed.Rows) != len(want) {
		t.Fatalf("expected %d rows, got %d", len(want), len(parsed.Rows))
	}
	for i, row := range parsed.Rows {
		if row.Subscribed != want[i] {
			t.Errorf("row %d (%s): got subscribed=%v, want %v", i, row.Email, row.Subscribed, want[i])
		}
	}
}

func TestParseContactsCSVRejectsTooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("email\n")
	for i := 0; i < maxImportRows+1; i++ {
		b.WriteString("person@example.com\n")
	}
	if _, err := ParseContactsCSV(strings.NewReader(b.String())); err == nil {
		t.Fatal("expected an error for a CSV exceeding the row cap")
	}
}

func TestIsValidEmail(t *testing.T) {
	valid := []string{"a@example.com", "first.last@sub.example.co.uk"}
	invalid := []string{"", "not-an-email", "a@", "@example.com", "a b@example.com"}

	for _, email := range valid {
		if !IsValidEmail(email) {
			t.Errorf("expected %q to be valid", email)
		}
	}
	for _, email := range invalid {
		if IsValidEmail(email) {
			t.Errorf("expected %q to be invalid", email)
		}
	}
}
