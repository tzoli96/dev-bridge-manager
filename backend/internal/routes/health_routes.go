package routes

import (
	"dev-bridge-manager/internal/handlers"
	"github.com/gofiber/fiber/v2"
)

func SetupHealthRoutes(api fiber.Router) {
	healthHandler := handlers.NewHealthHandler()

	// Health endpoints
	api.Get("/health", healthHandler.Check)
	api.Get("/ping", healthHandler.Ping)
}
