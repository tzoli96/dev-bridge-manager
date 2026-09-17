// backend/internal/routes/invoice_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInvoiceRoutes(api fiber.Router) {
	invoiceHandler := handlers.NewInvoiceHandler()

	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// POST /api/v1/projects/:id/invoices - Számla kiállítása egy projekthez
	projects.Post("/:id/invoices", invoiceHandler.CreateInvoice)

	// GET /api/v1/projects/:id/invoices - Projekt számla-előzményeinek listázása
	projects.Get("/:id/invoices", invoiceHandler.GetProjectInvoices)
}
