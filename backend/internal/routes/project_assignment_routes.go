package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProjectAssignmentRoutes(api fiber.Router) {
	assignmentHandler := handlers.NewProjectAssignmentHandler()

	// Project assignment routes group
	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// GET /api/v1/projects/:id/assignments - Projekt hozzárendelések listázása
	projects.Get("/:id/assignments", assignmentHandler.GetProjectAssignments)

	// POST /api/v1/projects/:id/assignments - User hozzárendelése projekthez
	projects.Post("/:id/assignments", assignmentHandler.AssignUserToProject)

	// PUT /api/v1/projects/:id/assignments/:user_id - Hozzárendelés módosítása
	projects.Put("/:id/assignments/:user_id", assignmentHandler.UpdateProjectAssignment)

	// DELETE /api/v1/projects/:id/assignments/:user_id - User eltávolítása projektből
	projects.Delete("/:id/assignments/:user_id", assignmentHandler.RemoveUserFromProject)
}
