// backend/internal/services/gmail_sync_helpers_test.go
package services

import (
	"errors"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestClassifyFolder(t *testing.T) {
	cases := []struct {
		name       string
		labels     []string
		wantFolder string
		wantOK     bool
	}{
		{"inbox", []string{"INBOX", "UNREAD"}, "inbox", true},
		{"sent", []string{"SENT"}, "sent", true},
		{"sent takes priority over inbox", []string{"INBOX", "SENT"}, "sent", true},
		{"draft only is skipped", []string{"DRAFT"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			folder, ok := classifyFolder(tc.labels)
			if folder != tc.wantFolder || ok != tc.wantOK {
				t.Errorf("classifyFolder(%v) = (%q, %v), want (%q, %v)", tc.labels, folder, ok, tc.wantFolder, tc.wantOK)
			}
		})
	}
}

func TestPendingMessageIDs(t *testing.T) {
	fetched := []string{"a", "b", "c"}
	existing := map[string]bool{"b": true}

	got := pendingMessageIDs(fetched, existing)

	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("pendingMessageIDs = %v, want [a c]", got)
	}
}

func TestIsHistoryExpiredError(t *testing.T) {
	notFound := &googleapi.Error{Code: 404}
	if !isHistoryExpiredError(notFound) {
		t.Error("expected 404 googleapi.Error to be treated as history-expired")
	}

	forbidden := &googleapi.Error{Code: 403}
	if isHistoryExpiredError(forbidden) {
		t.Error("expected 403 googleapi.Error not to be treated as history-expired")
	}

	if isHistoryExpiredError(errors.New("plain error")) {
		t.Error("expected a non-googleapi error to be false")
	}
}

func TestIsAuthError(t *testing.T) {
	unauthorized := &googleapi.Error{Code: 401}
	if !isAuthError(unauthorized) {
		t.Error("expected 401 googleapi.Error to be treated as an auth error")
	}
	if isAuthError(errors.New("plain error")) {
		t.Error("expected a non-googleapi error to be false")
	}
}

func TestSplitNameAddress(t *testing.T) {
	name, addr := splitNameAddress(`"Jane Doe" <jane@example.com>`)
	if name != "Jane Doe" || addr != "jane@example.com" {
		t.Errorf("got (%q, %q)", name, addr)
	}

	name, addr = splitNameAddress("plain@example.com")
	if name != "" || addr != "plain@example.com" {
		t.Errorf("got (%q, %q), want (\"\", plain@example.com)", name, addr)
	}
}
