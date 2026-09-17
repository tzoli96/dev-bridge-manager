// backend/internal/services/gmail_sync.go
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const gmailSyncInterval = 3 * time.Hour
const gmailInitialBackfillDays = 30

// StartGmailSyncScheduler mirrors services.StartAutoInvoiceScheduler's plain
// time.Ticker pattern: no cron dependency exists in this codebase, and a
// 3-hour cadence tolerates the occasional missed tick from a restart.
func StartGmailSyncScheduler() {
	api := NewRealGmailAPI()
	RunGmailSync(api)

	ticker := time.NewTicker(gmailSyncInterval)
	for range ticker.C {
		RunGmailSync(api)
	}
}

func RunGmailSync(api GmailAPI) {
	var accounts []models.GmailAccount
	if err := database.GetDB().Find(&accounts).Error; err != nil {
		log.Printf("gmail sync: failed to load accounts: %v", err)
		return
	}
	for i := range accounts {
		if err := syncAccount(context.Background(), api, &accounts[i]); err != nil {
			log.Printf("gmail sync: account %d failed: %v", accounts[i].ID, err)
		}
	}
}

func syncAccount(ctx context.Context, api GmailAPI, account *models.GmailAccount) error {
	db := database.GetDB()

	if account.LastHistoryID == "" {
		cutoff := time.Now().AddDate(0, 0, -gmailInitialBackfillDays)
		if err := backfillAccount(ctx, api, db, account, cutoff); err != nil {
			return recordSyncFailure(db, account, err)
		}
	} else {
		added, deleted, newHistoryID, err := api.ListHistory(ctx, account, account.LastHistoryID)
		if err != nil {
			if isHistoryExpiredError(err) {
				cutoff := time.Now()
				if account.LastSyncedAt != nil {
					cutoff = *account.LastSyncedAt
				}
				if err := backfillAccount(ctx, api, db, account, cutoff); err != nil {
					return recordSyncFailure(db, account, err)
				}
			} else {
				return recordSyncFailure(db, account, err)
			}
		} else {
			if err := applyAddedMessages(ctx, api, db, account, added); err != nil {
				return recordSyncFailure(db, account, err)
			}
			if len(deleted) > 0 {
				db.Where("gmail_account_id = ? AND gmail_message_id IN ?", account.ID, deleted).Delete(&models.Email{})
			}
			db.Model(account).Updates(map[string]interface{}{"last_history_id": newHistoryID, "needs_reauth": false})
			account.LastHistoryID = newHistoryID
		}
	}

	now := time.Now()
	db.Model(account).Update("last_synced_at", now)
	account.LastSyncedAt = &now
	return nil
}

// recordSyncFailure flags the account for reconnect when the failure is an
// auth error, then returns err unchanged so the caller still logs it.
func recordSyncFailure(db *gorm.DB, account *models.GmailAccount, err error) error {
	if isAuthError(err) {
		db.Model(account).Update("needs_reauth", true)
	}
	return err
}

func backfillAccount(ctx context.Context, api GmailAPI, db *gorm.DB, account *models.GmailAccount, after time.Time) error {
	query := fmt.Sprintf("after:%d", after.Unix())
	ids, err := api.ListMessageIDs(ctx, account, query)
	if err != nil {
		return err
	}

	if err := applyAddedMessages(ctx, api, db, account, ids); err != nil {
		return err
	}

	historyID, err := api.GetProfileHistoryID(ctx, account)
	if err != nil {
		return err
	}
	db.Model(account).Updates(map[string]interface{}{"last_history_id": historyID, "needs_reauth": false})
	account.LastHistoryID = historyID
	return nil
}

func applyAddedMessages(ctx context.Context, api GmailAPI, db *gorm.DB, account *models.GmailAccount, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	var existing []string
	db.Model(&models.Email{}).
		Where("gmail_account_id = ? AND gmail_message_id IN ?", account.ID, ids).
		Pluck("gmail_message_id", &existing)
	existingSet := make(map[string]bool, len(existing))
	for _, id := range existing {
		existingSet[id] = true
	}

	for _, id := range pendingMessageIDs(ids, existingSet) {
		meta, err := api.GetMessageMetadata(ctx, account, id)
		if err != nil {
			log.Printf("gmail sync: failed to fetch message %s: %v", id, err)
			continue
		}
		if meta == nil {
			continue // skipped: no INBOX/SENT label (see classifyFolder)
		}

		db.Create(&models.Email{
			GmailAccountID: account.ID,
			GmailMessageID: meta.GmailMessageID,
			ThreadID:       meta.ThreadID,
			Folder:         meta.Folder,
			FromAddress:    meta.FromAddress,
			FromName:       meta.FromName,
			ToAddresses:    meta.ToAddresses,
			Subject:        meta.Subject,
			Snippet:        meta.Snippet,
			HasAttachments: len(meta.Attachments) > 0,
			AttachmentMeta: models.AttachmentMetaToJSON(meta.Attachments),
			IsRead:         meta.IsRead,
			ReceivedAt:     meta.ReceivedAt,
			SyncedAt:       time.Now(),
		})
	}
	return nil
}
