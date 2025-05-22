package routes

import (
	"dev-bridge-manager/internal/handlers"
	"github.com/gofiber/fiber/v2"
)

func SetupUsersRoutes(api fiber.Router) {
	usersHandler := handlers.NewUsersHandler()

	users := api.Group("/users")
	users.Get("/", usersHandler.GetUsers)
	users.Get("/:id", usersHandler.GetUser)
}
