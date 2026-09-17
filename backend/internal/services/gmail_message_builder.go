// backend/internal/services/gmail_message_builder.go
package services

import (
	"fmt"
	"strings"
)

// BuildRawMessage builds an RFC 2822 message ready for GmailAPI.SendMessage.
// When inReplyToHeader is non-empty, In-Reply-To/References headers are set
// so Gmail threads the reply under the original message.
func BuildRawMessage(fromAddress, to, subject, body, inReplyToHeader, referencesHeader string) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	if inReplyToHeader != "" {
		sb.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyToHeader))
		refs := strings.TrimSpace(referencesHeader + " " + inReplyToHeader)
		sb.WriteString(fmt.Sprintf("References: %s\r\n", refs))
	}
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	sb.WriteString(body)
	return []byte(sb.String())
}
