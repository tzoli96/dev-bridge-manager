// backend/internal/routes/billingo_settings_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupBillingoSettingsRoutes(api fiber.Router) {
	billingoSettingsHandler := handlers.NewBillingoSettingsHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// GET /api/v1/admin/billingo-settings - Billingo beállítások lekérése (maszkolt API kulccsal)
	admin.Get("/billingo-settings", billingoSettingsHandler.GetBillingoSettings)

	// PUT /api/v1/admin/billingo-settings - Billingo beállítások frissítése
	admin.Put("/billingo-settings", billingoSettingsHandler.UpdateBillingoSettings)

	// GET /api/v1/admin/billingo-settings/blocks - Billingo számlatömbök lekérése
	admin.Get("/billingo-settings/blocks", billingoSettingsHandler.ListBillingoBlocks)
}
