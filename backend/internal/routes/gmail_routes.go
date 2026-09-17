package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupGmailRoutes(api fiber.Router) {
	h := handlers.NewGmailAuthHandler()
	gmail := api.Group("/gmail")

	// Public: Google's browser redirect lands here directly, carrying no
	// app Authorization header. Protected by the one-time state param
	// (see services.consumeOAuthState) instead of JWT.
	gmail.Get("/callback", h.Callback)

	gmail.Use(middleware.JWTMiddleware())
	gmail.Get("/auth-url", middleware.RequirePermission("gmail.manage"), h.AuthURL)
	gmail.Get("/status", middleware.RequirePermission("gmail.manage"), h.Status)
	gmail.Post("/disconnect", middleware.RequirePermission("gmail.manage"), h.Disconnect)
}
