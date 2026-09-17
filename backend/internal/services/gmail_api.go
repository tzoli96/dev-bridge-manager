// backend/internal/services/gmail_api.go
package services

import (
	"context"
	"time"

	"dev-bridge-manager/internal/models"
)

// GmailMessageMeta is the metadata-only shape stored per synced email.
type GmailMessageMeta struct {
	GmailMessageID string
	ThreadID       string
	Folder         string // "inbox" | "sent"
	FromAddress    string
	FromName       string
	ToAddresses    string
	Subject        string
	Snippet        string
	IsRead         bool
	ReceivedAt     time.Time
	Attachments    []models.EmailAttachmentMeta
}

// GmailFullMessage is fetched live (never persisted) when a user opens an
// email or replies to one.
type GmailFullMessage struct {
	GmailMessageMeta
	BodyText         string
	BodyHTML         string
	MessageIDHeader  string
	ReferencesHeader string
}

// GmailAPI is the seam between the sync/handler logic and the real Gmail
// SDK, so gmail_sync.go and email_handler.go can be exercised with a fake
// in tests without hitting Google's servers.
type GmailAPI interface {
	ListMessageIDs(ctx context.Context, account *models.GmailAccount, query string) ([]string, error)
	GetMessageMetadata(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailMessageMeta, error)
	GetFullMessage(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailFullMessage, error)
	GetProfileHistoryID(ctx context.Context, account *models.GmailAccount) (string, error)
	ListHistory(ctx context.Context, account *models.GmailAccount, startHistoryID string) (added []string, deleted []string, newHistoryID string, err error)
	GetAttachment(ctx context.Context, account *models.GmailAccount, messageID, attachmentID string) (data []byte, err error)
	SendMessage(ctx context.Context, account *models.GmailAccount, raw []byte) (gmailMessageID string, err error)
}
