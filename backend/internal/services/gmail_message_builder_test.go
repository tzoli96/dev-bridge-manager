// backend/internal/services/gmail_message_builder_test.go
package services

import (
	"encoding/base64"
	"mime"
	"strings"
	"testing"
)

func TestBuildRawMessage_NonASCIISubjectAndBody(t *testing.T) {
	subject := "Számla értesítő - Teszt"
	body := "Kedves Ügyfél!\n\nÜdvözlettel,\ncsapat"

	raw := BuildRawMessage("from@example.com", "to@example.com", subject, body, "", "", "", nil)

	headerPart, bodyPart, found := strings.Cut(string(raw), "\r\n\r\n")
	if !found {
		t.Fatalf("expected a blank-line header/body separator in message: %q", string(raw))
	}

	var subjectLine string
	for _, line := range strings.Split(headerPart, "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			subjectLine = strings.TrimPrefix(line, "Subject: ")
		}
	}
	if subjectLine == "" {
		t.Fatalf("Subject header not found in headers: %q", headerPart)
	}

	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(subjectLine)
	if err != nil {
		t.Fatalf("failed to RFC 2047-decode Subject header %q: %v", subjectLine, err)
	}
	if decodedSubject != subject {
		t.Errorf("Subject round-trip = %q, want %q", decodedSubject, subject)
	}

	if !strings.Contains(headerPart, "Content-Transfer-Encoding: base64") {
		t.Errorf("expected a Content-Transfer-Encoding: base64 header, got headers: %q", headerPart)
	}

	decodedBody, err := base64.StdEncoding.DecodeString(bodyPart)
	if err != nil {
		t.Fatalf("failed to base64-decode body %q: %v", bodyPart, err)
	}
	if string(decodedBody) != body {
		t.Errorf("body round-trip = %q, want %q", string(decodedBody), body)
	}
}

func TestBuildRawMessage_WithAttachment(t *testing.T) {
	body := "See attached."
	fileContent := []byte("%PDF-1.4 fake pdf content")
	attachments := []MessageAttachment{
		{Filename: "invoice.pdf", ContentType: "application/pdf", Data: fileContent},
	}

	raw := BuildRawMessage("from@example.com", "to@example.com", "Invoice", body, "", "", "", attachments)
	rawStr := string(raw)

	if !strings.Contains(rawStr, "Content-Type: multipart/mixed; boundary=") {
		t.Fatalf("expected multipart/mixed Content-Type header, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, `Content-Disposition: attachment; filename="invoice.pdf"`) {
		t.Errorf("expected Content-Disposition header for the attachment, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, base64.StdEncoding.EncodeToString(fileContent)) {
		t.Errorf("expected base64-encoded attachment content in message, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, base64.StdEncoding.EncodeToString([]byte(body))) {
		t.Errorf("expected base64-encoded body in message, got: %q", rawStr)
	}
}

func TestBuildRawMessage_NoAttachmentsStillSimple(t *testing.T) {
	raw := BuildRawMessage("from@example.com", "to@example.com", "Hi", "body", "", "", "", nil)
	if strings.Contains(string(raw), "multipart/mixed") {
		t.Errorf("expected simple text/plain message without attachments, got multipart: %q", string(raw))
	}
}

func TestBuildRawMessage_WithHTMLBody(t *testing.T) {
	bodyText := "Plain fallback"
	bodyHTML := "<p><strong>Formatted</strong> body</p>"

	raw := BuildRawMessage("from@example.com", "to@example.com", "Hi", bodyText, bodyHTML, "", "", nil)
	rawStr := string(raw)

	if !strings.Contains(rawStr, "Content-Type: multipart/alternative; boundary=") {
		t.Fatalf("expected multipart/alternative Content-Type header, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, "Content-Type: text/plain") || !strings.Contains(rawStr, "Content-Type: text/html") {
		t.Errorf("expected both text/plain and text/html parts, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, base64.StdEncoding.EncodeToString([]byte(bodyText))) {
		t.Errorf("expected base64-encoded plain body in message, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, base64.StdEncoding.EncodeToString([]byte(bodyHTML))) {
		t.Errorf("expected base64-encoded HTML body in message, got: %q", rawStr)
	}
}

func TestBuildRawMessage_WithHTMLBodyAndAttachment(t *testing.T) {
	bodyText := "Plain fallback"
	bodyHTML := "<p>Formatted</p>"
	attachments := []MessageAttachment{
		{Filename: "invoice.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4 fake pdf content")},
	}

	raw := BuildRawMessage("from@example.com", "to@example.com", "Hi", bodyText, bodyHTML, "", "", attachments)
	rawStr := string(raw)

	if !strings.Contains(rawStr, "Content-Type: multipart/mixed; boundary=") {
		t.Fatalf("expected multipart/mixed Content-Type header, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, "Content-Type: multipart/alternative; boundary=") {
		t.Errorf("expected a nested multipart/alternative part, got: %q", rawStr)
	}
	if !strings.Contains(rawStr, `Content-Disposition: attachment; filename="invoice.pdf"`) {
		t.Errorf("expected the attachment part to still be present, got: %q", rawStr)
	}
}
