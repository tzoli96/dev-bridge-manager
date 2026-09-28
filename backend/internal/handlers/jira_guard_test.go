// backend/internal/handlers/jira_guard_test.go
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIsJiraSourced(t *testing.T) {
	if isJiraSourced(models.Task{Source: "local"}) {
		t.Error("expected local task to not be Jira-sourced")
	}
	if !isJiraSourced(models.Task{Source: "jira"}) {
		t.Error("expected jira task to be Jira-sourced")
	}
}

func TestIsJiraColumn(t *testing.T) {
	if isJiraColumn(models.KanbanColumn{}) {
		t.Error("expected column with nil JiraStatusName to not be a Jira column")
	}
	statusName := "To Do"
	if !isJiraColumn(models.KanbanColumn{JiraStatusName: &statusName}) {
		t.Error("expected column with JiraStatusName set to be a Jira column")
	}
}
