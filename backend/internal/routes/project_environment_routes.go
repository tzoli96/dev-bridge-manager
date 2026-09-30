// backend/internal/routes/project_environment_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProjectEnvironmentRoutes(api fiber.Router) {
	environmentHandler := handlers.NewProjectEnvironmentHandler()

	environments := api.Group("/projects/:id/environments")
	environments.Use(middleware.JWTMiddleware())
	environments.Use(middleware.RequireProjectMember())

	// GET /api/v1/projects/:id/environments - Projekt környezeteinek listázása
	environments.Get("/", environmentHandler.GetProjectEnvironments)

	// POST /api/v1/projects/:id/environments - Új környezet létrehozása
	environments.Post("/", environmentHandler.CreateProjectEnvironment)

	// PUT /api/v1/projects/:id/environments/:environmentId - Környezet frissítése
	environments.Put("/:environmentId", environmentHandler.UpdateProjectEnvironment)

	// DELETE /api/v1/projects/:id/environments/:environmentId - Környezet törlése
	environments.Delete("/:environmentId", environmentHandler.DeleteProjectEnvironment)
}
