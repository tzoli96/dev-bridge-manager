// backend/internal/routes/search_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupSearchRoutes(api fiber.Router) {
	h := handlers.NewSearchHandler()

	// GET /api/v1/search?q=... - Cégszintű gyorskeresés (ügyfelek, projektek, feladatok)
	api.Get("/search", middleware.JWTMiddleware(), h.Search)
}
