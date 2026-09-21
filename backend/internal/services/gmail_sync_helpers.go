// backend/internal/services/gmail_sync_helpers.go
package services

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"strings"

	"dev-bridge-manager/internal/models"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

// classifyFolder maps a Gmail message's labelIds to this app's two-folder
// model. A message with both SENT and INBOX (e.g. replying to yourself)
// is classified as "sent" since that's the action the user took. Messages
// with neither label (drafts, pure spam/trash) are skipped.
func classifyFolder(labelIDs []string) (folder string, ok bool) {
	hasInbox, hasSent := false, false
	for _, l := range labelIDs {
		if l == "INBOX" {
			hasInbox = true
		}
		if l == "SENT" {
			hasSent = true
		}
	}
	switch {
	case hasSent:
		return "sent", true
	case hasInbox:
		return "inbox", true
	default:
		return "", false
	}
}

// categorizeIfInbox asks the categorizer for meta's category when meta is
// an inbox message. On any categorizer error, or for non-inbox messages,
// it returns nil rather than failing — the sync must not stop just because
// the AI service is unreachable or returned something unexpected (see
// EmailCategorizationService.Categorize's own validation).
func categorizeIfInbox(ctx context.Context, categorizer EmailCategorizer, meta *GmailMessageMeta) *string {
	if categorizer == nil {
		return nil
	}
	if meta.Folder != "inbox" {
		return nil
	}
	category, err := categorizer.Categorize(ctx, meta.Subject, meta.Snippet, meta.FromAddress, meta.FromName)
	if err != nil {
		log.Printf("gmail sync: failed to categorize message %s: %v", meta.GmailMessageID, err)
		return nil
	}
	return category
}

// pendingMessageIDs returns the ids in fetched that are not already in
// existingIDs, preserving fetched's order. Used by the sync job to avoid
// re-fetching/re-inserting messages already mirrored locally.
func pendingMessageIDs(fetched []string, existingIDs map[string]bool) []string {
	var out []string
	for _, id := range fetched {
		if !existingIDs[id] {
			out = append(out, id)
		}
	}
	return out
}

// isHistoryExpiredError reports whether err is Gmail's 404 for a
// startHistoryId that's aged out of Gmail's history buffer, which requires
// falling back to a date-query resync.
func isHistoryExpiredError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}

// isAuthError reports whether err means the stored refresh token is no
// longer valid (revoked access), which should mark the account as needing
// the user to reconnect rather than retrying. Besides a direct 401 from a
// Gmail API call, a revoked refresh token also surfaces as an
// *oauth2.RetrieveError during TokenSource.Token() (i.e. before any Gmail
// API call is made), so that path is checked too.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 401
	}
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		return strings.Contains(string(retrieveErr.Body), "invalid_grant")
	}
	return false
}

// splitNameAddress parses a From/To header value like `"Jane Doe" <jane@x.com>`
// into (name, address). If there's no display name, name is "".
func splitNameAddress(header string) (name, address string) {
	header = strings.TrimSpace(header)
	if idx := strings.LastIndex(header, "<"); idx != -1 && strings.HasSuffix(header, ">") {
		name = strings.Trim(header[:idx], ` "`)
		address = strings.TrimSuffix(header[idx+1:], ">")
		return name, address
	}
	return "", header
}

func containsLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// extractAttachmentMeta walks a message payload's parts recursively,
// collecting parts that are real attachments (have a filename and an
// attachment id) as opposed to inline text/html body parts.
func extractAttachmentMeta(part *gmail.MessagePart) []models.EmailAttachmentMeta {
	if part == nil {
		return nil
	}
	var out []models.EmailAttachmentMeta
	if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
		out = append(out, models.EmailAttachmentMeta{
			Filename:     part.Filename,
			Size:         part.Body.Size,
			AttachmentID: part.Body.AttachmentId,
		})
	}
	for _, child := range part.Parts {
		out = append(out, extractAttachmentMeta(child)...)
	}
	return out
}

// extractBody walks a message payload for the first text/plain and
// text/html parts. Gmail's payload body data is base64url-encoded.
func extractBody(part *gmail.MessagePart) (text, html string) {
	if part == nil {
		return "", ""
	}
	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		text = decodeBase64URL(part.Body.Data)
	}
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
		html = decodeBase64URL(part.Body.Data)
	}
	for _, child := range part.Parts {
		childText, childHTML := extractBody(child)
		if text == "" {
			text = childText
		}
		if html == "" {
			html = childHTML
		}
	}
	return text, html
}

// decodeBase64URL decodes Gmail's base64url message body data. Gmail is
// documented to omit padding, but some messages come back with trailing
// "=" padding anyway, so any is stripped before decoding with the
// no-padding decoder rather than assuming one form or the other.
func decodeBase64URL(s string) string {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return ""
	}
	return string(b)
}
