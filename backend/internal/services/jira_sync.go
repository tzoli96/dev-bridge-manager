// backend/internal/services/jira_sync.go
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

const jiraSyncInterval = 15 * time.Minute

// StartJiraSyncScheduler mirrors gmail_sync.go's StartGmailSyncScheduler
// pattern: run once immediately, then on a fixed ticker.
func StartJiraSyncScheduler() {
	client := NewRealJiraClient()
	RunJiraSync(client)

	ticker := time.NewTicker(jiraSyncInterval)
	for range ticker.C {
		RunJiraSync(client)
	}
}

// RunJiraSync syncs every connected board in turn, tolerating individual
// board failures the same way RunGmailSync tolerates individual account
// failures - one board's error doesn't stop the others from syncing.
func RunJiraSync(client JiraClient) {
	var integrations []models.JiraBoardIntegration
	if err := database.GetDB().Find(&integrations).Error; err != nil {
		log.Printf("jira sync: failed to load integrations: %v", err)
		return
	}
	for i := range integrations {
		if err := syncBoardIntegration(context.Background(), client, &integrations[i]); err != nil {
			log.Printf("jira sync: board %d failed: %v", integrations[i].BoardID, err)
		}
	}
}

func syncBoardIntegration(ctx context.Context, client JiraClient, integration *models.JiraBoardIntegration) error {
	db := database.GetDB()

	var board models.Board
	if err := db.First(&board, integration.BoardID).Error; err != nil {
		return fmt.Errorf("loading board: %w", err)
	}

	issues, err := client.SearchAssignedIssues(ctx, *integration)
	if err != nil {
		db.Model(integration).Updates(map[string]interface{}{"last_sync_at": time.Now(), "last_sync_error": err.Error()})
		return err
	}

	var columns []models.KanbanColumn
	db.Where("board_id = ?", integration.BoardID).Find(&columns)
	columnByStatus := make(map[string]models.KanbanColumn, len(columns))
	for _, c := range columns {
		if c.JiraStatusName != nil {
			columnByStatus[*c.JiraStatusName] = c
		}
	}

	currentKeys := make([]string, 0, len(issues))
	for _, issue := range issues {
		currentKeys = append(currentKeys, issue.Key)

		column, ok := columnByStatus[issue.StatusName]
		if !ok {
			statusName := issue.StatusName
			column = models.KanbanColumn{
				BoardID:        integration.BoardID,
				Title:          statusName,
				Color:          "bg-blue-500",
				Position:       nextColumnPosition(columns),
				JiraStatusName: &statusName,
			}
			if err := db.Create(&column).Error; err != nil {
				log.Printf("jira sync: board %d failed to create column for status %q: %v", integration.BoardID, statusName, err)
				continue
			}
			columns = append(columns, column)
			columnByStatus[statusName] = column
		}

		var task models.Task
		lookupErr := db.Where("project_id = ? AND jira_issue_key = ?", board.ProjectID, issue.Key).First(&task).Error
		now := time.Now()

		if lookupErr == gorm.ErrRecordNotFound {
			jiraKey := issue.Key
			task = models.Task{
				ProjectID:    board.ProjectID,
				Title:        issue.Summary,
				Description:  issue.Description,
				Priority:     "medium",
				Status:       "todo",
				Source:       "jira",
				JiraIssueKey: &jiraKey,
				JiraSyncedAt: &now,
				CreatedBy:    integration.ConnectedBy,
				UpdatedBy:    integration.ConnectedBy,
			}
			if err := db.Create(&task).Error; err != nil {
				log.Printf("jira sync: board %d failed to create task for issue %s: %v", integration.BoardID, issue.Key, err)
				continue
			}

			var maxPosition struct{ Max int }
			db.Model(&models.TaskPlacement{}).
				Select("COALESCE(MAX(position), -1) as max").
				Where("column_id = ?", column.ID).
				Scan(&maxPosition)
			placement := models.TaskPlacement{TaskID: task.ID, BoardID: integration.BoardID, ColumnID: column.ID, Position: maxPosition.Max + 1}
			if err := db.Create(&placement).Error; err != nil {
				log.Printf("jira sync: board %d failed to place task for issue %s: %v", integration.BoardID, issue.Key, err)
			}
		} else if lookupErr != nil {
			log.Printf("jira sync: board %d failed to look up task for issue %s: %v", integration.BoardID, issue.Key, lookupErr)
			continue
		} else {
			task.Title = issue.Summary
			task.Description = issue.Description
			task.JiraSyncedAt = &now
			db.Save(&task)

			var placement models.TaskPlacement
			if err := db.Where("task_id = ? AND board_id = ?", task.ID, integration.BoardID).First(&placement).Error; err == nil {
				if placement.ColumnID != column.ID {
					placement.ColumnID = column.ID
					db.Save(&placement)
				}
			}
		}
	}

	var mirroredKeys []string
	db.Model(&models.Task{}).
		Where("project_id = ? AND source = ? AND jira_issue_key IS NOT NULL", board.ProjectID, "jira").
		Pluck("jira_issue_key", &mirroredKeys)

	for _, key := range issueKeysToRemove(mirroredKeys, currentKeys) {
		db.Where("project_id = ? AND jira_issue_key = ?", board.ProjectID, key).Delete(&models.Task{})
	}

	db.Model(integration).Updates(map[string]interface{}{"last_sync_at": time.Now(), "last_sync_error": ""})
	return nil
}

// issueKeysToRemove returns the previously-mirrored issue keys that no longer
// appear in the current search results, meaning they've left the "assigned,
// not yet done" set (completed or reassigned) and should be un-mirrored.
func issueKeysToRemove(mirroredKeys []string, currentKeys []string) []string {
	current := make(map[string]bool, len(currentKeys))
	for _, k := range currentKeys {
		current[k] = true
	}
	var toRemove []string
	for _, k := range mirroredKeys {
		if !current[k] {
			toRemove = append(toRemove, k)
		}
	}
	return toRemove
}

// nextColumnPosition returns the position a newly auto-created Jira-status
// column should take: one past the highest existing position on the board.
func nextColumnPosition(existing []models.KanbanColumn) int {
	max := -1
	for _, c := range existing {
		if c.Position > max {
			max = c.Position
		}
	}
	return max + 1
}
