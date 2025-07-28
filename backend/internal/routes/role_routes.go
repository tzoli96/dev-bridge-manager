package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupRoleRoutes(api fiber.Router) {
	roleHandler := handlers.NewRoleHandler()

	roles := api.Group("/roles")

	// All role endpoints require authentication
	roles.Use(middleware.JWTMiddleware())

	// Role management (admin only)
	roles.Get("/",
		middleware.RequirePermission("roles.list"),
		roleHandler.GetAllRoles,
	)

	roles.Post("/",
		middleware.RequirePermission("roles.create"),
		roleHandler.CreateRole,
	)

	roles.Put("/:id",
		middleware.RequirePermission("roles.update"),
		roleHandler.UpdateRole,
	)
}
