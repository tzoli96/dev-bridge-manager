package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupUsersRoutes(api fiber.Router) {
	usersHandler := handlers.NewUsersHandler()

	users := api.Group("/users")
	users.Get("/", usersHandler.GetUsers)
	users.Get("/:id", usersHandler.GetUser)

	userHandler := handlers.NewUserHandler()

	users.Use(middleware.JWTMiddleware())

	// User endpoints with permission-based access
	users.Get("/",
		middleware.RequirePermission("users.list"),
		userHandler.GetAllUsers,
	)

	users.Post("/",
		middleware.RequirePermission("users.create"),
		userHandler.CreateUser,
	)

	users.Get("/:id",
		middleware.RequireAnyPermission("users.read", "profile.read"),
		userHandler.GetUser,
	)

	users.Put("/:id",
		middleware.RequireAnyPermission("users.update", "profile.update"),
		userHandler.UpdateUser,
	)

	users.Delete("/:id",
		middleware.RequirePermission("users.delete"),
		userHandler.DeleteUser,
	)

	// Profile endpoints
	users.Get("/profile/me",
		middleware.RequirePermission("profile.read"),
		userHandler.GetProfile,
	)
}
