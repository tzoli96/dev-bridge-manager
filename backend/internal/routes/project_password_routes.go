// backend/internal/routes/project_password_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProjectPasswordRoutes(api fiber.Router) {
	passwordHandler := handlers.NewProjectPasswordHandler()

	passwords := api.Group("/projects/:id/passwords")
	passwords.Use(middleware.JWTMiddleware())
	passwords.Use(middleware.RequireProjectMember())

	// GET /api/v1/projects/:id/passwords - Projekt jelszavainak listázása
	passwords.Get("/", passwordHandler.GetProjectPasswords)

	// POST /api/v1/projects/:id/passwords - Új jelszó létrehozása
	passwords.Post("/", passwordHandler.CreateProjectPassword)

	// PUT /api/v1/projects/:id/passwords/:passwordId - Jelszó frissítése
	passwords.Put("/:passwordId", passwordHandler.UpdateProjectPassword)

	// DELETE /api/v1/projects/:id/passwords/:passwordId - Jelszó törlése
	passwords.Delete("/:passwordId", passwordHandler.DeleteProjectPassword)
}
