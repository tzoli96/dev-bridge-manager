// backend/internal/services/gmail_message_builder.go
package services

import (
	"encoding/base64"
	"fmt"
	"mime"
	"strings"

	"github.com/google/uuid"
)

// MessageAttachment is a file to embed in an outgoing message via
// BuildRawMessage.
type MessageAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// BuildRawMessage builds an RFC 2822 message ready for GmailAPI.SendMessage.
// When inReplyToHeader is non-empty, In-Reply-To/References headers are set
// so Gmail threads the reply under the original message. When bodyHTML is
// non-empty, the message carries both the plain-text and HTML bodies as a
// multipart/alternative part (HTML-capable clients render bodyHTML, others
// fall back to bodyText). When attachments is non-empty, the message is
// wrapped in multipart/mixed with the body (plain or alternative) as the
// first part and each attachment base64-encoded as a subsequent part.
func BuildRawMessage(fromAddress, to, subject, bodyText, bodyHTML, inReplyToHeader, referencesHeader string, attachments []MessageAttachment) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject)))
	if inReplyToHeader != "" {
		sb.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyToHeader))
		refs := strings.TrimSpace(referencesHeader + " " + inReplyToHeader)
		sb.WriteString(fmt.Sprintf("References: %s\r\n", refs))
	}
	sb.WriteString("MIME-Version: 1.0\r\n")

	hasHTML := bodyHTML != ""

	if len(attachments) == 0 && !hasHTML {
		sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		sb.WriteString("Content-Transfer-Encoding: base64\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(base64.StdEncoding.EncodeToString([]byte(bodyText)))
		return []byte(sb.String())
	}

	if len(attachments) == 0 {
		boundary := "boundary-" + uuid.NewString()
		sb.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		sb.WriteString("\r\n")
		writeAlternativeParts(&sb, boundary, bodyText, bodyHTML)
		sb.WriteString(fmt.Sprintf("--%s--", boundary))
		return []byte(sb.String())
	}

	boundary := "boundary-" + uuid.NewString()
	sb.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	sb.WriteString("\r\n")

	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	if hasHTML {
		altBoundary := "boundary-" + uuid.NewString()
		sb.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", altBoundary))
		sb.WriteString("\r\n")
		writeAlternativeParts(&sb, altBoundary, bodyText, bodyHTML)
		sb.WriteString(fmt.Sprintf("--%s--\r\n", altBoundary))
	} else {
		sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		sb.WriteString("Content-Transfer-Encoding: base64\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(base64.StdEncoding.EncodeToString([]byte(bodyText)))
		sb.WriteString("\r\n")
	}

	for _, a := range attachments {
		contentType := a.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		sb.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, a.Filename))
		sb.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", a.Filename))
		sb.WriteString("Content-Transfer-Encoding: base64\r\n")
		sb.WriteString("\r\n")
		sb.WriteString(base64.StdEncoding.EncodeToString(a.Data))
		sb.WriteString("\r\n")
	}
	sb.WriteString(fmt.Sprintf("--%s--", boundary))

	return []byte(sb.String())
}

// writeAlternativeParts writes a multipart/alternative body pair (plain
// text first, then HTML) nested under the given boundary. Callers write the
// boundary's Content-Type header and closing delimiter themselves.
func writeAlternativeParts(sb *strings.Builder, boundary, bodyText, bodyHTML string) {
	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	sb.WriteString("Content-Transfer-Encoding: base64\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(base64.StdEncoding.EncodeToString([]byte(bodyText)))
	sb.WriteString("\r\n")

	sb.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	sb.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	sb.WriteString("Content-Transfer-Encoding: base64\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(base64.StdEncoding.EncodeToString([]byte(bodyHTML)))
	sb.WriteString("\r\n")
}
