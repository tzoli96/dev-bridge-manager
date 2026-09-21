// backend/internal/routes/profile_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProfileRoutes(api fiber.Router) {
	h := handlers.NewProfileHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only, deliberately no admin fallback - the profile holds
	// personal background/writing-style data the operator wants to keep to
	// themselves. Passed per-route (not via admin.Use()) so it doesn't leak
	// onto other /admin/* routes registered by other route files - see
	// profile_handler.go's doc comment for why that matters.
	admin.Get("/profile", middleware.RequireRole("super_admin"), h.GetProfile)
	admin.Put("/profile", middleware.RequireRole("super_admin"), h.UpdateProfile)

	internal := api.Group("/internal")
	internal.Get("/profile-context", h.GetProfileContext)
}
