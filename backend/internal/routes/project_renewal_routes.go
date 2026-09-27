// backend/internal/routes/project_renewal_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProjectRenewalRoutes(api fiber.Router) {
	h := handlers.NewProjectRenewalHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only for now, same reasoning as kanban_stall_routes.go -
	// passed per-route so it never leaks onto other /admin/* routes.
	admin.Get("/project-renewal-flags", middleware.RequireRole("super_admin"), h.ListProjectRenewalFlags)
	admin.Post("/project-renewal-flags/:id/dismiss", middleware.RequireRole("super_admin"), h.Dismiss)
}
