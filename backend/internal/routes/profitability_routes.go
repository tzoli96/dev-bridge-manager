// backend/internal/routes/profitability_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProfitabilityRoutes(api fiber.Router) {
	h := handlers.NewProfitabilityHandler()

	g := api.Group("/profitability")
	g.Use(middleware.JWTMiddleware())
	g.Use(middleware.RequirePermission("profitability.read"))

	// GET /api/v1/profitability/overview - Ügyfél- és projekt-óradíjak (névleges és valódi)
	g.Get("/overview", h.GetOverview)

	// GET /api/v1/profitability/settings - Levelezés-becslés beállításai
	g.Get("/settings", h.GetSettings)

	// PUT /api/v1/profitability/settings - Beállítások mentése
	g.Put("/settings", middleware.RequirePermission("profitability.manage"), h.UpdateSettings)

	// PUT /api/v1/profitability/clients/:id/meeting-allowance - Havi megbeszélés-átalány
	g.Put("/clients/:id/meeting-allowance", middleware.RequirePermission("profitability.manage"), h.PutMeetingAllowance)
}
