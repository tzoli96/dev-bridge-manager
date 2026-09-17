package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupClientRoutes(api fiber.Router) {
	clientHandler := handlers.NewClientHandler()

	// Client routes group
	clients := api.Group("/clients")

	// Minden client route-hoz autentikáció szükséges
	clients.Use(middleware.JWTMiddleware())

	// PUBLIC ENDPOINTS (minden bejelentkezett user számára)
	// GET /api/v1/clients - Ügyfelek listázása
	clients.Get("/", clientHandler.GetAllClients)

	// GET /api/v1/clients/:id - Egy ügyfél megtekintése
	clients.Get("/:id", clientHandler.GetClient)

	// ADMIN ONLY ENDPOINTS
	// POST /api/v1/clients - Ügyfél létrehozása (csak admin)
	clients.Post("/", clientHandler.CreateClient)

	// PUT /api/v1/clients/:id - Ügyfél frissítése (csak admin)
	clients.Put("/:id", clientHandler.UpdateClient)

	// DELETE /api/v1/clients/:id - Ügyfél törlése (csak admin)
	clients.Delete("/:id", clientHandler.DeleteClient)
}
