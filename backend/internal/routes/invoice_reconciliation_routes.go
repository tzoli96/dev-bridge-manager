// backend/internal/routes/invoice_reconciliation_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInvoiceReconciliationRoutes(api fiber.Router) {
	h := handlers.NewInvoiceReconciliationHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only for now, same reasoning as kanban_stall_routes.go -
	// passed per-route so it never leaks onto other /admin/* routes.
	admin.Get("/invoice-reconciliation-flags", middleware.RequireRole("super_admin"), h.ListInvoiceReconciliationFlags)
	admin.Post("/invoice-reconciliation-flags/:id/dismiss", middleware.RequireRole("super_admin"), h.Dismiss)
}
