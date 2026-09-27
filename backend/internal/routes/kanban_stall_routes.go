// backend/internal/routes/kanban_stall_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupKanbanStallRoutes(api fiber.Router) {
	h := handlers.NewKanbanStallHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only for now, same reasoning as job_search_routes.go -
	// passed per-route so it never leaks onto other /admin/* routes.
	admin.Get("/kanban-stall-flags", middleware.RequireRole("super_admin"), h.ListKanbanStallFlags)
	admin.Post("/kanban-stall-flags/:id/dismiss", middleware.RequireRole("super_admin"), h.Dismiss)
}
