package routes

import "github.com/gofiber/fiber/v2"

func SetupRoutes(app *fiber.App) {

	// API versioning
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Health endpoints at root level
	SetupHealthRoutes(v1)

	// Setup route groups
	SetupAPIRoutes(v1)        // API endpoints under /api/v1
	SetupAuthRoutes(v1)       // Auth endpoints
	SetupUsersRoutes(v1)      // Users endpoints
	SetupRoleRoutes(v1)       // Role endpoints
	SetupPermissionRoutes(v1) // Permission endpoints
}

func SetupAPIRoutes(api fiber.Router) {
	// API info endpoint
	api.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Dev Bridge Manager API v1",
			"endpoints": []string{
				"GET /health - Health check",
				"GET /ping - Ping test",
				"GET /api/v1 - API info",
				"GET /api/v1/users - Get all users",
				"GET /api/v1/users/:id - Get user by ID",
				"POST /api/v1/auth/register - User registration",
				"POST /api/v1/auth/login - User login",
				"GET /api/v1/auth/me - Current user (protected)",
				"GET /api/v1/users - Get all users",
				"GET /api/v1/users/:id - Get user by ID",
			},
		})
	})
}
