// backend/internal/routes/activity_digest_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupActivityDigestRoutes(api fiber.Router) {
	h := handlers.NewActivityDigestHandler()

	// GET /api/v1/activity-digest?date=YYYY-MM-DD - Napi tevékenység összefoglaló
	api.Get("/activity-digest", middleware.JWTMiddleware(), h.GetDigest)
}
