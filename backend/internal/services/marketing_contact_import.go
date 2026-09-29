// backend/internal/services/marketing_contact_import.go
package services

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// maxImportRows is the hard cap on data rows a single marketing-contacts
// CSV import may contain. The import runs synchronously within one HTTP
// request (no background job), so an unbounded file could tie up a
// request-handling goroutine indefinitely.
const maxImportRows = 20000

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// IsValidEmail reports whether email passes a basic format check (an
// "@" followed later by a ".", no whitespace) - not a full RFC 5322
// validator, just enough to catch obviously malformed rows without a
// new dependency.
func IsValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

// ContactRow is one successfully parsed, valid data row from an
// imported CSV. RowNumber is the file's line number (the header is row
// 1, so the first data row is row 2), used to point the user at the
// right line if a later write fails.
type ContactRow struct {
	RowNumber  int
	Email      string
	FirstName  string
	LastName   string
	Subscribed bool
	Tags       string
	Source     string
	Notes      string
}

// ParsedContacts is ParseContactsCSV's result. Rows holds every valid
// data row; Errors holds one message per row that was skipped for
// being invalid (never a structural failure - those are returned as an
// error instead).
type ParsedContacts struct {
	Rows   []ContactRow
	Errors []string
}

// ParseContactsCSV reads a marketing-contacts CSV. The header row is
// required and its columns are matched by name (case-insensitive,
// order doesn't matter): email, first_name, last_name, subscribed,
// tags, source, notes. Unknown extra columns are ignored.
//
// It returns a non-nil error only for structural problems - a missing
// "email" column, an unparseable file, or more than maxImportRows data
// rows - so the caller can reject the whole request before writing
// anything. Per-row problems (empty or malformed email) do not fail
// the whole import: that row is left out of Rows and a message is
// appended to Errors instead, so the caller can still process every
// other row.
func ParseContactsCSV(r io.Reader) (ParsedContacts, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return ParsedContacts{}, fmt.Errorf("could not read CSV header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}

	colIndex := make(map[string]int, len(header))
	for i, name := range header {
		colIndex[strings.ToLower(strings.TrimSpace(name))] = i
	}
	if _, ok := colIndex["email"]; !ok {
		return ParsedContacts{}, errors.New(`CSV header is missing an "email" column`)
	}

	cell := func(record []string, col string) string {
		i, ok := colIndex[col]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	var result ParsedContacts
	rowNumber := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParsedContacts{}, fmt.Errorf("could not read CSV row %d: %w", rowNumber+1, err)
		}
		rowNumber++

		if rowNumber-1 > maxImportRows {
			return ParsedContacts{}, fmt.Errorf("CSV has more than %d data rows", maxImportRows)
		}

		email := strings.ToLower(cell(record, "email"))
		if email == "" || !IsValidEmail(email) {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: invalid or missing email", rowNumber))
			continue
		}

		result.Rows = append(result.Rows, ContactRow{
			RowNumber:  rowNumber,
			Email:      email,
			FirstName:  cell(record, "first_name"),
			LastName:   cell(record, "last_name"),
			Subscribed: parseSubscribed(cell(record, "subscribed")),
			Tags:       cell(record, "tags"),
			Source:     cell(record, "source"),
			Notes:      cell(record, "notes"),
		})
	}

	return result, nil
}

// parseSubscribed maps a CSV cell's text to a bool: "true"/"1"/"yes"
// (case-insensitive) are true, everything else - including an empty
// cell - is false. This matches the literal "true"/"false" strings
// ExportMarketingContacts writes, so an exported file re-imports
// cleanly, while still tolerating a hand-edited or differently-cased
// file.
func parseSubscribed(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}
