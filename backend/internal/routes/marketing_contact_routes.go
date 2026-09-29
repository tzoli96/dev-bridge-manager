// backend/internal/routes/marketing_contact_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupMarketingContactRoutes(api fiber.Router) {
	h := handlers.NewMarketingContactHandler()

	contacts := api.Group("/marketing-contacts")
	contacts.Use(middleware.JWTMiddleware())

	// GET /api/v1/marketing-contacts - Kontaktok listázása (kereséssel/szűréssel)
	contacts.Get("/", h.ListMarketingContacts)

	// POST /api/v1/marketing-contacts - Új kontakt létrehozása
	contacts.Post("/", h.CreateMarketingContact)

	// POST /api/v1/marketing-contacts/import - CSV import
	contacts.Post("/import", h.ImportMarketingContacts)

	// GET /api/v1/marketing-contacts/export - CSV export
	contacts.Get("/export", h.ExportMarketingContacts)

	// PUT /api/v1/marketing-contacts/:id - Kontakt szerkesztése
	contacts.Put("/:id", h.UpdateMarketingContact)

	// DELETE /api/v1/marketing-contacts/:id - Kontakt törlése
	contacts.Delete("/:id", h.DeleteMarketingContact)
}
