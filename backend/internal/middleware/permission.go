package middleware

import (
	"dev-bridge-manager/internal/services"
	"github.com/gofiber/fiber/v2"
)

// RequirePermission middleware that checks for specific permission
func RequirePermission(permissionName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(uint)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"message": "Authentication required",
			})
		}

		permissionService := services.NewPermissionService()
		hasPermission, err := permissionService.CheckUserPermission(userID, permissionName)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"message": "Failed to check permissions",
			})
		}

		if !hasPermission {
			return c.Status(403).JSON(fiber.Map{
				"success":             false,
				"message":             "Insufficient permissions",
				"required_permission": permissionName,
			})
		}

		return c.Next()
	}
}

// RequireAnyPermission middleware that checks for any of the specified permissions
func RequireAnyPermission(permissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(uint)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"message": "Authentication required",
			})
		}

		permissionService := services.NewPermissionService()
		user, err := permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"message": "Failed to check permissions",
			})
		}

		hasAnyPermission := false
		for _, permission := range permissions {
			if user.HasPermission(permission) {
				hasAnyPermission = true
				break
			}
		}

		if !hasAnyPermission {
			return c.Status(403).JSON(fiber.Map{
				"success":              false,
				"message":              "Insufficient permissions",
				"required_permissions": permissions,
			})
		}

		return c.Next()
	}
}

// RequireRole middleware (compatibility)
func RequireRole(roleName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(uint)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"message": "Authentication required",
			})
		}

		permissionService := services.NewPermissionService()
		user, err := permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"message": "Failed to check role",
			})
		}

		if !user.HasRole(roleName) {
			return c.Status(403).JSON(fiber.Map{
				"success":       false,
				"message":       "Insufficient permissions",
				"required_role": roleName,
				"user_role":     user.Role.Name,
			})
		}

		return c.Next()
	}
}
