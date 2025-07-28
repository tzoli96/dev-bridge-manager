package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupUsersRoutes(api fiber.Router) {
	usersHandler := handlers.NewUsersHandler()
	userHandler := handlers.NewUserHandler()

	// PUBLIC ENDPOINTS (middleware nélkül)
	api.Get("/users", usersHandler.GetUsers)    // GET /api/v1/users (public lista)
	api.Get("/users/:id", usersHandler.GetUser) // GET /api/v1/users/123 (public user)

	// PROTECTED ENDPOINTS (külön group)
	protected := api.Group("/users", middleware.JWTMiddleware())

	// Profile endpoints
	protected.Get("/profile/me",
		middleware.RequirePermission("profile.read"),
		userHandler.GetProfile,
	)

	protected.Put("/profile/me",
		middleware.RequirePermission("profile.update"),
		userHandler.UpdateProfile,
	)

	protected.Put("/profile/password",
		middleware.RequirePermission("profile.update"),
		userHandler.ChangePassword,
	)

	// Admin CRUD műveletek
	protected.Post("/",
		middleware.RequirePermission("users.create"),
		userHandler.CreateUser,
	)

	protected.Put("/:id",
		middleware.RequirePermission("users.update"),
		userHandler.UpdateUser,
	)

	protected.Delete("/:id",
		middleware.RequirePermission("users.delete"),
		userHandler.DeleteUser,
	)

	// Admin user részletek (ha kell külön védett verzió)
	protected.Get("/:id/details",
		middleware.RequirePermission("users.read"),
		userHandler.GetUser,
	)
}
