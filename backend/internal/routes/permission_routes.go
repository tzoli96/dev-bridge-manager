package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupPermissionRoutes(api fiber.Router) {
	permissionHandler := handlers.NewPermissionHandler()

	permissions := api.Group("/permissions")
	permissions.Use(middleware.JWTMiddleware())

	// Get all permissions (admin only)
	permissions.Get("/",
		middleware.RequirePermission("system.settings"),
		permissionHandler.GetAllPermissions,
	)

	// Get current user's permissions
	permissions.Get("/me",
		middleware.RequirePermission("profile.read"),
		permissionHandler.GetUserPermissions,
	)
}
