// backend/internal/services/invoice_notice.go
package services

import (
	"context"
	"fmt"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// BuildInvoiceNoticeText composes the Hungarian subject/body for the
// pre-invoice notice e-mail. Pure function so it can be unit-tested without a
// fake Gmail server.
func BuildInvoiceNoticeText(clientName, projectName string, periodStart, periodEnd *time.Time) (subject, body string) {
	periodText := ""
	if periodStart != nil && periodEnd != nil {
		periodText = fmt.Sprintf(" a %s - %s időszakra vonatkozóan", periodStart.Format("2006.01.02"), periodEnd.Format("2006.01.02"))
	}
	subject = fmt.Sprintf("Számla értesítő - %s", projectName)
	body = fmt.Sprintf(
		"Kedves %s!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"%s\" projekt kapcsán%s.\n\nÜdvözlettel",
		clientName, projectName, periodText,
	)
	return subject, body
}

// SendInvoiceNoticeEmail sends the pre-invoice notice e-mail from the given
// Gmail account to the client, then persists the resulting 'pending'
// InvoiceNotice row. Used both by the manual "Értesítő küldése" button
// (InvoiceNoticeHandler.SendInvoiceNotice) and by the monthly scheduler
// (RunAutoInvoiceNotices), which has no logged-in user to send from — hence
// account/sentBy are passed in explicitly rather than read from request
// context.
func SendInvoiceNoticeEmail(project models.Project, client models.Client, account models.GmailAccount, periodStart, periodEnd *time.Time, sentBy uint) (*models.InvoiceNotice, error) {
	subject, body := BuildInvoiceNoticeText(client.Name, project.Name, periodStart, periodEnd)
	raw := BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "", "", nil)
	gmailMessageID, err := NewRealGmailAPI().SendMessage(context.Background(), &account, raw)
	if err != nil {
		return nil, err
	}

	notice := models.InvoiceNotice{
		ProjectID:      project.ID,
		ClientID:       client.ID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		GmailMessageID: gmailMessageID,
		SentBy:         sentBy,
		SentAt:         time.Now(),
		Status:         "pending",
	}
	if err := database.GetDB().Create(&notice).Error; err != nil {
		return nil, err
	}
	return &notice, nil
}

// SendInvoiceReadyEmail notifies the client that their invoice has been
// issued, with the Billingo PDF attached. Sent automatically right after
// ApproveInvoiceNotice creates the invoice, and also available as a manual
// (re-)send via InvoiceHandler.SendInvoiceEmail for any created invoice.
func SendInvoiceReadyEmail(account models.GmailAccount, client models.Client, project models.Project, invoiceNumber string, pdfBytes []byte) error {
	subject := fmt.Sprintf("Számla - %s", project.Name)
	body := fmt.Sprintf(
		"Kedves %s!\n\nMellékelten küldjük a(z) \"%s\" projekt %s számú számláját.\n\nÜdvözlettel",
		client.Name, project.Name, invoiceNumber,
	)
	attachments := []MessageAttachment{{
		Filename:    invoiceNumber + ".pdf",
		ContentType: "application/pdf",
		Data:        pdfBytes,
	}}
	raw := BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "", "", attachments)
	_, err := NewRealGmailAPI().SendMessage(context.Background(), &account, raw)
	return err
}
