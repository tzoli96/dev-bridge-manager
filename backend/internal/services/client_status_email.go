// backend/internal/services/client_status_email.go
package services

import (
	"context"
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

const clientStatusEmailCheckInterval = 1 * time.Hour

// RunClientStatusEmailDrafts drafts one pending ClientStatusEmail per client
// with weekly activity, for the calendar week that just ended (Monday
// through the prior Sunday). Idempotent per client/period: a client already
// drafted for this exact period is skipped, so re-running on the same
// Monday (e.g. from the hourly ticker) is harmless.
func RunClientStatusEmailDrafts(drafter ClientStatusDrafter) {
	summaries, start, end, err := listClientWeeklySummariesForWeekBeforeNow()
	if err != nil {
		log.Printf("⚠️ Client status emails: failed to summarize week: %v", err)
		return
	}

	db := database.GetDB()
	for _, summary := range summaries {
		var count int64
		db.Model(&models.ClientStatusEmail{}).
			Where("client_id = ? AND period_start = ? AND period_end = ?", summary.ClientID, start, end).
			Count(&count)
		if count > 0 {
			continue
		}

		facts := ClientStatusFacts{
			ClientName:           summary.ClientName,
			PeriodStart:          start.Format("2006-01-02"),
			PeriodEnd:            end.Format("2006-01-02"),
			CompletedTasks:       summary.CompletedTasks,
			HoursLogged:          summary.HoursLogged,
			InvoicesCreated:      summary.InvoicesCreated,
			InvoicesCreatedTotal: summary.InvoicesCreatedTotal,
			InvoicesPaid:         summary.InvoicesPaid,
		}
		subject, body, err := drafter.DraftClientStatusEmail(context.Background(), facts)
		if err != nil {
			log.Printf("⚠️ Client status emails: failed to draft email for client %d: %v", summary.ClientID, err)
			continue
		}

		draft := models.ClientStatusEmail{
			ClientID:    summary.ClientID,
			PeriodStart: start,
			PeriodEnd:   end,
			Subject:     subject,
			Body:        body,
			Status:      "pending",
			CreatedAt:   time.Now(),
		}
		if err := db.Create(&draft).Error; err != nil {
			log.Printf("⚠️ Client status emails: failed to save draft for client %d: %v", summary.ClientID, err)
		}
	}
}

// StartClientStatusEmailScheduler mirrors scheduler.go's day-gated ticker
// pattern: runs once at startup (in case the server was down on Monday) and
// then hourly for as long as today is Monday.
func StartClientStatusEmailScheduler() {
	checkAndRunClientStatusEmails()

	ticker := time.NewTicker(clientStatusEmailCheckInterval)
	for range ticker.C {
		checkAndRunClientStatusEmails()
	}
}

func checkAndRunClientStatusEmails() {
	if time.Now().Weekday() != time.Monday {
		return
	}
	log.Println("📧 Client status emails: drafting weekly summaries")
	RunClientStatusEmailDrafts(NewClientStatusAIService())
}
