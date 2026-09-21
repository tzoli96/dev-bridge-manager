// backend/internal/models/email.go
package models

import (
	"encoding/json"
	"time"
)

type EmailAttachmentMeta struct {
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	AttachmentID string `json:"attachment_id"`
}

// EmailCategory* constants and ValidEmailCategories are the canonical list
// of categories the AI service (ai/app/categorize_email.py) can return -
// kept in sync manually since the two live in different languages.
const (
	EmailCategoryClient    = "ugyfel"
	EmailCategoryBilling   = "szamla"
	EmailCategoryMarketing = "marketing"
	EmailCategorySystem    = "rendszeruzenet"
	EmailCategoryOther     = "egyeb"
)

var ValidEmailCategories = map[string]bool{
	EmailCategoryClient:    true,
	EmailCategoryBilling:   true,
	EmailCategoryMarketing: true,
	EmailCategorySystem:    true,
	EmailCategoryOther:     true,
}

type Email struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	GmailAccountID uint      `json:"gmail_account_id" gorm:"not null"`
	GmailMessageID string    `json:"gmail_message_id" gorm:"size:100;not null"`
	ThreadID       string    `json:"thread_id" gorm:"size:100"`
	Folder         string    `json:"folder" gorm:"size:10;not null"` // "inbox" | "sent"
	FromAddress    string    `json:"from_address" gorm:"size:255"`
	FromName       string    `json:"from_name" gorm:"size:255"`
	ToAddresses    string    `json:"to_addresses" gorm:"size:1000"`
	Subject        string    `json:"subject" gorm:"size:998"`
	Snippet        string    `json:"snippet" gorm:"size:1000"`
	HasAttachments bool      `json:"has_attachments"`
	AttachmentMeta string    `json:"-" gorm:"type:jsonb;default:'[]'"`
	IsRead         bool      `json:"is_read"`
	ReceivedAt     time.Time `json:"received_at"`
	SyncedAt       time.Time `json:"synced_at"`
	Category       *string   `json:"category" gorm:"size:20"`
}

func (Email) TableName() string { return "emails" }

// Attachments unmarshals AttachmentMeta, following the Tags/TagsFromJSON
// precedent in models/kanban.go. Returns nil (not an error) on bad JSON,
// since AttachmentMeta is always written by AttachmentMetaToJSON.
func (e Email) Attachments() []EmailAttachmentMeta {
	var out []EmailAttachmentMeta
	_ = json.Unmarshal([]byte(e.AttachmentMeta), &out)
	return out
}

func AttachmentMetaToJSON(items []EmailAttachmentMeta) string {
	if len(items) == 0 {
		return "[]"
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

type EmailListItem struct {
	ID             uint                  `json:"id"`
	Folder         string                `json:"folder"`
	FromAddress    string                `json:"from_address"`
	FromName       string                `json:"from_name"`
	ToAddresses    string                `json:"to_addresses"`
	Subject        string                `json:"subject"`
	Snippet        string                `json:"snippet"`
	HasAttachments bool                  `json:"has_attachments"`
	Attachments    []EmailAttachmentMeta `json:"attachments"`
	IsRead         bool                  `json:"is_read"`
	ReceivedAt     time.Time             `json:"received_at"`
	Category       *string               `json:"category"`
}

type EmailListResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Emails  []EmailListItem `json:"emails,omitempty"`
	Total   int64           `json:"total,omitempty"`
}

type EmailDetailResponse struct {
	Success     bool                  `json:"success"`
	Message     string                `json:"message,omitempty"`
	ID          uint                  `json:"id,omitempty"`
	Folder      string                `json:"folder,omitempty"`
	Subject     string                `json:"subject,omitempty"`
	From        string                `json:"from,omitempty"`
	To          string                `json:"to,omitempty"`
	BodyText    string                `json:"body_text,omitempty"`
	BodyHTML    string                `json:"body_html,omitempty"`
	Attachments []EmailAttachmentMeta `json:"attachments,omitempty"`
	ReceivedAt  time.Time             `json:"received_at,omitempty"`
}

type EmailSendRequest struct {
	To               string `json:"to"`
	Subject          string `json:"subject"`
	Body             string `json:"body"`
	InReplyToEmailID uint   `json:"in_reply_to_email_id,omitempty"` // local emails.id being replied to
}

type EmailSendResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message,omitempty"`
	GmailMessageID string `json:"gmail_message_id,omitempty"`
}
