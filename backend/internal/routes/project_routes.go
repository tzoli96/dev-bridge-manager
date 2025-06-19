package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProjectRoutes(api fiber.Router) {
	projectHandler := handlers.NewProjectHandler()

	// Project routes group
	projects := api.Group("/projects")

	// Minden project route-hoz autentikáció szükséges
	projects.Use(middleware.JWTMiddleware())

	// PUBLIC ENDPOINTS (minden bejelentkezett user számára)
	// GET /api/v1/projects - Projektek listázása (mindenki láthatja)
	projects.Get("/", projectHandler.GetAllProjects)

	// GET /api/v1/projects/:id - Egy projekt megtekintése (mindenki láthatja)
	projects.Get("/:id", projectHandler.GetProject)

	// ADMIN ONLY ENDPOINTS
	// POST /api/v1/projects - Projekt létrehozása (csak admin)
	// Az admin jogosultság ellenőrzés a handler-ben történik
	projects.Post("/", projectHandler.CreateProject)

	// PUT /api/v1/projects/:id - Projekt frissítése (csak admin)
	// Az admin jogosultság ellenőrzés a handler-ben történik
	projects.Put("/:id", projectHandler.UpdateProject)

	// DELETE /api/v1/projects/:id - Projekt törlése (csak admin)
	// Az admin jogosultság ellenőrzés a handler-ben történik
	projects.Delete("/:id", projectHandler.DeleteProject)
}
