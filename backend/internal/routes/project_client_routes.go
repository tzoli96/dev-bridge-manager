package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProjectClientRoutes(api fiber.Router) {
	projectClientHandler := handlers.NewProjectClientHandler()

	// Project client routes group
	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// GET /api/v1/projects/:id/clients - Projekthez rendelt ügyfelek listázása
	projects.Get("/:id/clients", projectClientHandler.GetProjectClients)

	// POST /api/v1/projects/:id/clients - Ügyfél hozzárendelése projekthez
	projects.Post("/:id/clients", projectClientHandler.AssignClientToProject)

	// DELETE /api/v1/projects/:id/clients/:client_id - Ügyfél eltávolítása projektből
	projects.Delete("/:id/clients/:client_id", projectClientHandler.RemoveClientFromProject)
}
