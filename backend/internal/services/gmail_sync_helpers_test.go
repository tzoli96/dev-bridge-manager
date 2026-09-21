// backend/internal/services/gmail_sync_helpers_test.go
package services

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
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

	invalidGrant := &oauth2.RetrieveError{Body: []byte(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)}
	if !isAuthError(invalidGrant) {
		t.Error("expected an oauth2.RetrieveError with invalid_grant body to be treated as an auth error")
	}

	otherRetrieveErr := &oauth2.RetrieveError{Body: []byte(`{"error":"server_error"}`)}
	if isAuthError(otherRetrieveErr) {
		t.Error("expected an oauth2.RetrieveError without invalid_grant to be false")
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

type fakeCategorizer struct {
	called  bool
	gotArgs [4]string // subject, snippet, fromAddress, fromName
	result  *string
	err     error
}

func (f *fakeCategorizer) Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error) {
	f.called = true
	f.gotArgs = [4]string{subject, snippet, fromAddress, fromName}
	return f.result, f.err
}

func TestCategorizeIfInboxSkipsSentFolder(t *testing.T) {
	cat := &fakeCategorizer{}
	meta := &GmailMessageMeta{Folder: "sent", Subject: "hi"}

	got := categorizeIfInbox(context.Background(), cat, meta)

	if got != nil {
		t.Errorf("expected nil category for sent folder, got %v", got)
	}
	if cat.called {
		t.Error("expected categorizer not to be called for sent folder")
	}
}

func TestCategorizeIfInboxCallsCategorizerForInbox(t *testing.T) {
	category := "ugyfel"
	cat := &fakeCategorizer{result: &category}
	meta := &GmailMessageMeta{Folder: "inbox", Subject: "hi", Snippet: "snip", FromAddress: "a@b.com", FromName: "A"}

	got := categorizeIfInbox(context.Background(), cat, meta)

	if got == nil || *got != "ugyfel" {
		t.Fatalf("expected category 'ugyfel', got %v", got)
	}
	if cat.gotArgs != [4]string{"hi", "snip", "a@b.com", "A"} {
		t.Errorf("categorizer called with unexpected args: %+v", cat.gotArgs)
	}
}

func TestCategorizeIfInboxReturnsNilOnCategorizerError(t *testing.T) {
	cat := &fakeCategorizer{err: errors.New("ai service down")}
	meta := &GmailMessageMeta{Folder: "inbox", Subject: "hi"}

	got := categorizeIfInbox(context.Background(), cat, meta)

	if got != nil {
		t.Errorf("expected nil category on categorizer error, got %v", got)
	}
}
