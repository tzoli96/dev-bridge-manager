// backend/internal/routes/client_health_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupClientHealthRoutes(api fiber.Router) {
	h := handlers.NewClientHealthHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only for now, same reasoning as kanban_stall_routes.go -
	// passed per-route so it never leaks onto other /admin/* routes.
	admin.Get("/client-health", middleware.RequireRole("super_admin"), h.List)
}
