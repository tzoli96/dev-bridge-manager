package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupAuthRoutes(api fiber.Router) {
	authHandler := handlers.NewAuthHandler()

	auth := api.Group("/auth")

	// Public endpoints
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)

	// Protected endpoints
	auth.Get("/me", middleware.JWTMiddleware(), authHandler.Me)
	auth.Post("/refresh", authHandler.RefreshToken)

}
