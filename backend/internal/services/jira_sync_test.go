// backend/internal/services/jira_sync_test.go
package services

import (
	"reflect"
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIssueKeysToRemove(t *testing.T) {
	mirrored := []string{"PROJ-1", "PROJ-2", "PROJ-3"}
	current := []string{"PROJ-2", "PROJ-4"}
	got := issueKeysToRemove(mirrored, current)
	want := []string{"PROJ-1", "PROJ-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIssueKeysToRemove_NoneToRemove(t *testing.T) {
	got := issueKeysToRemove([]string{"PROJ-1"}, []string{"PROJ-1", "PROJ-2"})
	if len(got) != 0 {
		t.Errorf("expected no keys to remove, got %v", got)
	}
}

func TestNextColumnPosition_Empty(t *testing.T) {
	got := nextColumnPosition(nil)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestNextColumnPosition_NonEmpty(t *testing.T) {
	cols := []models.KanbanColumn{{Position: 0}, {Position: 2}, {Position: 1}}
	got := nextColumnPosition(cols)
	if got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}
