// backend/internal/services/project_renewal.go
package services

import (
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// projectRenewalLeadTime matches the approved design: a project whose
// contract_end_date is 30 days away or closer (including already past)
// counts as due for a renewal flag.
const projectRenewalLeadTime = 30 * 24 * time.Hour

// projectRenewalRepeatInterval mirrors kanbanStallRepeatInterval's
// dismiss-then-recheck pattern: a dismissed flag is reconsidered once this
// much time has passed and the contract still hasn't been renewed.
const projectRenewalRepeatInterval = 7 * 24 * time.Hour

// projectRenewalCheckInterval mirrors kanbanStallCheckInterval's daily
// cadence - a project's renewal date only needs to be re-evaluated once a day.
const projectRenewalCheckInterval = 24 * time.Hour

// renewalIsDue reports whether a project's contract_end_date is close
// enough (or already past) to warrant a flag. Pure function so it can be
// unit-tested without a database.
func renewalIsDue(contractEndDate time.Time, now time.Time) bool {
	return contractEndDate.Sub(now) <= projectRenewalLeadTime
}

// renewalFlagIsDue decides whether a new pending ProjectRenewalFlag should
// be created for a project, given the most recently created flag for it
// (nil if none exists yet). No flag is proposed while one is already
// pending, to avoid stacking duplicates; a dismissed flag is reconsidered
// once projectRenewalRepeatInterval has elapsed.
func renewalFlagIsDue(latest *models.ProjectRenewalFlag, now time.Time) bool {
	if latest == nil {
		return true
	}
	if latest.Status == "pending" {
		return false
	}
	return now.Sub(latest.CreatedAt) >= projectRenewalRepeatInterval
}

// RunProjectRenewalCheck flags every active project whose contract_end_date
// is within projectRenewalLeadTime (or already past). Pending flags for
// projects whose contract_end_date has since been pushed back out beyond
// the lead time (i.e. the contract was renewed) are cleaned up automatically
// instead of being left stale.
func RunProjectRenewalCheck() {
	db := database.GetDB()
	now := time.Now()

	var pending []models.ProjectRenewalFlag
	if err := db.Where("status = ?", "pending").Find(&pending).Error; err != nil {
		log.Printf("⚠️ Project renewal check: failed to load pending flags: %v", err)
		return
	}
	for _, flag := range pending {
		var project models.Project
		if err := db.First(&project, flag.ProjectID).Error; err != nil {
			continue
		}
		if project.ContractEndDate == nil || !renewalIsDue(*project.ContractEndDate, now) {
			db.Delete(&models.ProjectRenewalFlag{}, flag.ID)
		}
	}

	var projects []models.Project
	if err := db.Where("status = ? AND contract_end_date IS NOT NULL", "active").Find(&projects).Error; err != nil {
		log.Printf("⚠️ Project renewal check: failed to load projects: %v", err)
		return
	}

	for _, project := range projects {
		if project.ContractEndDate == nil || !renewalIsDue(*project.ContractEndDate, now) {
			continue
		}

		var latest models.ProjectRenewalFlag
		err := db.Where("project_id = ?", project.ID).Order("created_at DESC").First(&latest).Error
		var latestPtr *models.ProjectRenewalFlag
		if err == nil {
			latestPtr = &latest
		}

		if !renewalFlagIsDue(latestPtr, now) {
			continue
		}

		flag := models.ProjectRenewalFlag{
			ProjectID:       project.ID,
			ContractEndDate: *project.ContractEndDate,
			Status:          "pending",
			CreatedAt:       now,
		}
		if err := db.Create(&flag).Error; err != nil {
			log.Printf("⚠️ Project renewal check: failed to create flag for project %d: %v", project.ID, err)
		}
	}
}

// StartProjectRenewalScheduler mirrors StartKanbanStallScheduler's plain
// time.Ticker pattern: runs once at startup, then once every 24h.
func StartProjectRenewalScheduler() {
	log.Println("📄 Project renewal check: running")
	RunProjectRenewalCheck()

	ticker := time.NewTicker(projectRenewalCheckInterval)
	for range ticker.C {
		log.Println("📄 Project renewal check: running")
		RunProjectRenewalCheck()
	}
}
