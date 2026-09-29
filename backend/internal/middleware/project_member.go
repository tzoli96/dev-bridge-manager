// backend/internal/middleware/project_member.go
package middleware

import (
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

// isProjectMember reports whether a user with the given role name and
// (possibly nil) active project assignment should be treated as a
// member of a project: admins/super_admins always qualify; everyone
// else needs a non-nil assignment (any Role value on it - passwords
// give every active project member equal rights, per the approved
// design).
func isProjectMember(roleName string, assignment *models.ProjectAssignment) bool {
	if roleName == "admin" || roleName == "super_admin" {
		return true
	}
	return assignment != nil
}

// RequireProjectMember gates a /projects/:id/... route to users who are
// either an app-wide admin/super_admin, or have an active
// ProjectAssignment row for :id. Unlike RequirePermission/RequireRole,
// this checks membership in a *specific* project rather than a global
// role or permission - the first such check in this codebase (see the
// project password manager spec's "Important deviation" section).
func RequireProjectMember() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(uint)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"success": false, "message": "Authentication required"})
		}

		projectID, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
		}

		permissionService := services.NewPermissionService()
		user, err := permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to check project membership"})
		}

		var assignment *models.ProjectAssignment
		var found models.ProjectAssignment
		err = database.GetDB().
			Where("project_id = ? AND user_id = ? AND is_active = ?", projectID, userID, true).
			First(&found).Error
		if err == nil {
			assignment = &found
		}

		if !isProjectMember(user.Role.Name, assignment) {
			return c.Status(403).JSON(fiber.Map{"success": false, "message": "You are not a member of this project"})
		}

		return c.Next()
	}
}
