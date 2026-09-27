// backend/internal/routes/client_status_email_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupClientStatusEmailRoutes(api fiber.Router) {
	h := handlers.NewClientStatusEmailHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only, same reasoning as invoice_reconciliation_routes.go -
	// passed per-route so it never leaks onto other /admin/* routes.
	admin.Get("/client-status-emails", middleware.RequireRole("super_admin"), h.ListClientStatusEmails)
	admin.Post("/client-status-emails/:id/approve", middleware.RequireRole("super_admin"), h.Approve)
	admin.Post("/client-status-emails/:id/dismiss", middleware.RequireRole("super_admin"), h.Dismiss)
}
