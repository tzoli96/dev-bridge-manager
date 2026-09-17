// backend/internal/services/gmail_api_real.go
package services

import (
	"context"
	"encoding/base64"
	"strconv"
	"time"

	"dev-bridge-manager/internal/models"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type RealGmailAPI struct{}

func NewRealGmailAPI() *RealGmailAPI { return &RealGmailAPI{} }

func (a *RealGmailAPI) serviceFor(ctx context.Context, account *models.GmailAccount) (*gmail.Service, error) {
	token, err := validTokenForAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	return gmail.NewService(ctx, option.WithTokenSource(gmailOAuthConfig().TokenSource(ctx, token)))
}

func (a *RealGmailAPI) ListMessageIDs(ctx context.Context, account *models.GmailAccount, query string) ([]string, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	var ids []string
	pageToken := ""
	for {
		call := svc.Users.Messages.List("me").Q(query).MaxResults(500)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, m := range resp.Messages {
			ids = append(ids, m.Id)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return ids, nil
}

func (a *RealGmailAPI) GetMessageMetadata(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailMessageMeta, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	msg, err := svc.Users.Messages.Get("me", messageID).Format("metadata").
		MetadataHeaders("From", "To", "Subject").Do()
	if err != nil {
		return nil, err
	}

	folder, ok := classifyFolder(msg.LabelIds)
	if !ok {
		return nil, nil // caller (gmail_sync.go) skips messages with no INBOX/SENT label
	}

	meta := &GmailMessageMeta{
		GmailMessageID: msg.Id,
		ThreadID:       msg.ThreadId,
		Folder:         folder,
		Snippet:        msg.Snippet,
		IsRead:         !containsLabel(msg.LabelIds, "UNREAD"),
		ReceivedAt:     msTimeToTime(msg.InternalDate),
	}
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			meta.FromName, meta.FromAddress = splitNameAddress(h.Value)
		case "To":
			meta.ToAddresses = h.Value
		case "Subject":
			meta.Subject = h.Value
		}
	}
	meta.Attachments = extractAttachmentMeta(msg.Payload)
	return meta, nil
}

func (a *RealGmailAPI) GetFullMessage(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailFullMessage, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	msg, err := svc.Users.Messages.Get("me", messageID).Format("full").Do()
	if err != nil {
		return nil, err
	}

	folder, _ := classifyFolder(msg.LabelIds)
	full := &GmailFullMessage{
		GmailMessageMeta: GmailMessageMeta{
			GmailMessageID: msg.Id,
			ThreadID:       msg.ThreadId,
			Folder:         folder,
			Snippet:        msg.Snippet,
			ReceivedAt:     msTimeToTime(msg.InternalDate),
			Attachments:    extractAttachmentMeta(msg.Payload),
		},
	}
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			full.FromName, full.FromAddress = splitNameAddress(h.Value)
		case "To":
			full.ToAddresses = h.Value
		case "Subject":
			full.Subject = h.Value
		case "Message-ID":
			full.MessageIDHeader = h.Value
		case "References":
			full.ReferencesHeader = h.Value
		}
	}
	full.BodyText, full.BodyHTML = extractBody(msg.Payload)
	return full, nil
}

func (a *RealGmailAPI) GetProfileHistoryID(ctx context.Context, account *models.GmailAccount) (string, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return "", err
	}
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(profile.HistoryId, 10), nil
}

func (a *RealGmailAPI) ListHistory(ctx context.Context, account *models.GmailAccount, startHistoryID string) (added, deleted []string, newHistoryID string, err error) {
	svc, svcErr := a.serviceFor(ctx, account)
	if svcErr != nil {
		return nil, nil, "", svcErr
	}

	startID, parseErr := strconv.ParseUint(startHistoryID, 10, 64)
	if parseErr != nil {
		return nil, nil, "", parseErr
	}

	newHistoryID = startHistoryID
	pageToken := ""
	for {
		call := svc.Users.History.List("me").StartHistoryId(startID).
			HistoryTypes("messageAdded", "messageDeleted")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, callErr := call.Do()
		if callErr != nil {
			return nil, nil, "", callErr
		}

		for _, h := range resp.History {
			for _, m := range h.MessagesAdded {
				added = append(added, m.Message.Id)
			}
			for _, m := range h.MessagesDeleted {
				deleted = append(deleted, m.Message.Id)
			}
		}
		if resp.HistoryId != 0 {
			newHistoryID = strconv.FormatUint(resp.HistoryId, 10)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return added, deleted, newHistoryID, nil
}

func (a *RealGmailAPI) GetAttachment(ctx context.Context, account *models.GmailAccount, messageID, attachmentID string) ([]byte, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	att, err := svc.Users.Messages.Attachments.Get("me", messageID, attachmentID).Do()
	if err != nil {
		return nil, err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(att.Data)
}

func (a *RealGmailAPI) SendMessage(ctx context.Context, account *models.GmailAccount, raw []byte) (string, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return "", err
	}

	msg := &gmail.Message{Raw: base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(raw)}
	sent, err := svc.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return "", err
	}
	return sent.Id, nil
}

func msTimeToTime(ms int64) time.Time {
	return time.UnixMilli(ms)
}
