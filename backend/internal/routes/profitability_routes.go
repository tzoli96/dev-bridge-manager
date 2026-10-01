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

	// GET /api/v1/profitability/forecast - 3–6 hónapos bevétel-előrejelzés (biztos és megújítástól függő sáv)
	g.Get("/forecast", h.GetForecast)

	// GET /api/v1/profitability/parameter-sets - Mentett költség-paraméterkészletek
	g.Get("/parameter-sets", h.GetParameterSets)

	// POST /api/v1/profitability/parameter-sets - Új paraméterkészlet
	g.Post("/parameter-sets", middleware.RequirePermission("profitability.manage"), h.CreateParameterSet)

	// PUT /api/v1/profitability/parameter-sets/:id - Paraméterkészlet módosítása
	g.Put("/parameter-sets/:id", middleware.RequirePermission("profitability.manage"), h.UpdateParameterSet)

	// DELETE /api/v1/profitability/parameter-sets/:id - Törlés (409, ha forgatókönyv használja)
	g.Delete("/parameter-sets/:id", middleware.RequirePermission("profitability.manage"), h.DeleteParameterSet)

	// GET /api/v1/profitability/scenarios - Mentett forgatókönyvek
	g.Get("/scenarios", h.GetScenarios)

	// POST /api/v1/profitability/scenarios/compute - Mentés nélküli számolás (élő eredmény)
	g.Post("/scenarios/compute", h.ComputeScenario)

	// POST /api/v1/profitability/scenarios - Új forgatókönyv
	g.Post("/scenarios", middleware.RequirePermission("profitability.manage"), h.CreateScenario)

	// PUT /api/v1/profitability/scenarios/:id - Forgatókönyv módosítása
	g.Put("/scenarios/:id", middleware.RequirePermission("profitability.manage"), h.UpdateScenario)

	// DELETE /api/v1/profitability/scenarios/:id - Forgatókönyv törlése
	g.Delete("/scenarios/:id", middleware.RequirePermission("profitability.manage"), h.DeleteScenario)

	// GET /api/v1/profitability/settings - Levelezés-becslés beállításai
	g.Get("/settings", h.GetSettings)

	// PUT /api/v1/profitability/settings - Beállítások mentése
	g.Put("/settings", middleware.RequirePermission("profitability.manage"), h.UpdateSettings)

	// PUT /api/v1/profitability/clients/:id/meeting-allowance - Havi megbeszélés-átalány
	g.Put("/clients/:id/meeting-allowance", middleware.RequirePermission("profitability.manage"), h.PutMeetingAllowance)
}
