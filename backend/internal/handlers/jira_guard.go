// backend/internal/handlers/jira_guard.go
package handlers

import "dev-bridge-manager/internal/models"

// isJiraSourced reports whether a task is a read-only mirror of a Jira issue,
// meaning its content, placement, and lifecycle cannot be edited locally.
func isJiraSourced(task models.Task) bool {
	return task.Source == "jira"
}

// isJiraColumn reports whether a column was auto-created by the Jira sync job
// to mirror a Jira status, meaning it cannot be renamed, recolored, deleted,
// or reordered from the UI.
func isJiraColumn(column models.KanbanColumn) bool {
	return column.JiraStatusName != nil
}
