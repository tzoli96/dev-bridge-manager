// backend/internal/services/scheduler.go
package services

import (
	"log"
	"time"
)

// StartAutoInvoiceScheduler runs RunAutoInvoiceNotices once on startup (in
// case the server was down at midnight on the 1st) and then every hour for
// as long as today is the 1st of the month. There is no other background job
// runner in this codebase, so a plain ticker is used rather than adding a
// cron dependency; RunAutoInvoiceNotices is idempotent per project/period, so
// firing it multiple times on the 1st is harmless.
func StartAutoInvoiceScheduler() {
	checkAndRun()

	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		checkAndRun()
	}
}

func checkAndRun() {
	if time.Now().Day() != 1 {
		return
	}
	log.Println("🗓️ Auto-invoicing: running monthly check")
	RunAutoInvoiceNotices()
}
