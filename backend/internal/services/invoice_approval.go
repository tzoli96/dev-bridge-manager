// backend/internal/services/invoice_approval.go
package services

import (
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// defaultInvoiceDueDays matches the manual "Számla kiállítása" button's
// INVOICE_DUE_DAYS frontend constant (frontend/.../invoice/page.tsx), so
// approval-created invoices get the same payment deadline as manually
// created ones instead of falling back to Billingo's own default.
const defaultInvoiceDueDays = 8

// ApproveInvoiceNoticeResult carries everything a caller needs to build its
// own response (HTTP JSON or a scheduler log line) after approving a notice.
type ApproveInvoiceNoticeResult struct {
	Notice    models.InvoiceNotice
	Invoice   *models.Invoice
	Items     []models.InvoiceItem
	EmailSent bool
	Warning   string
}

// ApproveInvoiceNotice atomically claims a pending InvoiceNotice, creates the
// actual Billingo invoice for it (via CreateInvoiceForProject — the same
// logic the manual "Számla kiállítása" button uses), and best-effort e-mails
// the PDF to the client. Used both by InvoiceNoticeHandler.ApproveInvoiceNotice
// (a team member clicking "Jóváhagyás") and by the auto-invoice scheduler
// when a project has AutoInvoiceAutoApprove enabled, so the two paths can
// never diverge in how a notice turns into an invoice.
//
// approvedBy is recorded as the notice's approver and is whose Gmail account
// the PDF e-mail is sent from. httpStatus == 0 means success.
//
// The claim uses the same atomic UPDATE ... WHERE status = 'pending' pattern
// CreateInvoiceForProject uses to reserve a fixed-price invoice slot: a
// pre-check SELECT would leave a TOCTOU window open between the read and the
// later Updates call, letting a concurrent approval (or a client retry after
// an axios timeout) create two real Billingo invoices for one notice.
func ApproveInvoiceNotice(notice models.InvoiceNotice, project models.Project, client models.Client, approvedBy uint, billingoService *BillingoService) (result ApproveInvoiceNoticeResult, httpStatus int, message string) {
	db := database.GetDB()

	claimTime := time.Now()
	claim := db.Model(&models.InvoiceNotice{}).
		Where("id = ? AND status = ?", notice.ID, "pending").
		Updates(map[string]interface{}{"status": "approved", "approved_by": approvedBy, "approved_at": claimTime})
	if claim.Error != nil {
		return result, 500, "Failed to claim invoice notice"
	}
	if claim.RowsAffected == 0 {
		return result, 409, "Ez az értesítő már jóvá lett hagyva"
	}
	notice.Status = "approved"
	notice.ApprovedBy = &approvedBy
	notice.ApprovedAt = &claimTime

	req := models.InvoiceCreateRequest{ClientID: notice.ClientID}
	if notice.PeriodStart != nil {
		req.PeriodStart = notice.PeriodStart.Format("2006-01-02")
	}
	if notice.PeriodEnd != nil {
		req.PeriodEnd = notice.PeriodEnd.Format("2006-01-02")
	}
	req.DueDate = time.Now().AddDate(0, 0, defaultInvoiceDueDays).Format("2006-01-02")

	invoice, items, createStatus, createMessage := CreateInvoiceForProject(billingoService, project, client, req, approvedBy)
	if createStatus != 0 {
		// The notice was already claimed as "approved" above, but no invoice
		// was created — best-effort revert it back to "pending" so a retry is
		// possible instead of leaving it stuck.
		revert := db.Model(&models.InvoiceNotice{}).
			Where("id = ? AND status = ?", notice.ID, "approved").
			Updates(map[string]interface{}{"status": "pending", "approved_by": nil, "approved_at": nil})
		if revert.Error != nil {
			log.Printf("⚠️ Failed to revert invoice notice %d to pending after failed approval: %v", notice.ID, revert.Error)
		}
		return result, createStatus, createMessage
	}

	if err := db.Model(&models.InvoiceNotice{}).Where("id = ?", notice.ID).Update("invoice_id", invoice.ID).Error; err != nil {
		log.Printf("⚠️ Failed to set invoice_id on invoice notice %d: %v", notice.ID, err)
	}
	notice.InvoiceID = &invoice.ID

	result.Notice = notice
	result.Invoice = invoice
	result.Items = items

	var account models.GmailAccount
	if err := db.Where("user_id = ?", approvedBy).First(&account).Error; err != nil {
		result.Warning = "A számla elkészült, de nincs csatlakoztatott Gmail-fiókod — kérlek küldd el a PDF-et manuálisan."
		return result, 0, ""
	}
	var settings models.BillingoSettings
	if err := db.First(&settings, 1).Error; err != nil || settings.APIKey == "" {
		result.Warning = "A számla elkészült, de a Billingo nincs beállítva a PDF letöltéséhez — kérlek küldd el manuálisan."
		return result, 0, ""
	}
	pdfBytes, err := billingoService.DownloadInvoicePDF(settings.APIKey, invoice.BillingoInvoiceID)
	if err != nil {
		log.Printf("⚠️ Failed to download PDF for invoice %d: %v", invoice.ID, err)
		result.Warning = "A számla elkészült, de a PDF letöltése sikertelen — kérlek küldd el manuálisan."
		return result, 0, ""
	}
	if err := SendInvoiceReadyEmail(account, client, project, invoice.BillingoInvoiceNumber, pdfBytes); err != nil {
		log.Printf("⚠️ Failed to send invoice-ready e-mail for invoice %d: %v", invoice.ID, err)
		result.Warning = "A számla elkészült, de a PDF-es e-mail küldése sikertelen — kérlek küldd el manuálisan."
		return result, 0, ""
	}
	result.EmailSent = true
	return result, 0, ""
}
