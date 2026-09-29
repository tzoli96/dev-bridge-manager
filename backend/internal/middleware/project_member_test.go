// backend/internal/middleware/project_member_test.go
package middleware

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIsProjectMemberAdminAlwaysQualifies(t *testing.T) {
	if !isProjectMember("admin", nil) {
		t.Error("expected admin with no assignment to qualify as a project member")
	}
	if !isProjectMember("super_admin", nil) {
		t.Error("expected super_admin with no assignment to qualify as a project member")
	}
}

func TestIsProjectMemberRequiresAssignment(t *testing.T) {
	if isProjectMember("user", nil) {
		t.Error("expected a non-admin user with no assignment to not qualify")
	}
	if !isProjectMember("user", &models.ProjectAssignment{Role: "viewer"}) {
		t.Error("expected a non-admin user with an active assignment to qualify, regardless of assignment role")
	}
}
