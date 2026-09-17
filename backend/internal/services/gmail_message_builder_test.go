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

	raw := BuildRawMessage("from@example.com", "to@example.com", subject, body, "", "")

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
