package middleware

import (
	"dev-bridge-manager/internal/services"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func JWTMiddleware() fiber.Handler {
	authService := services.NewAuthService()

	return func(c *fiber.Ctx) error {
		// Get Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"message": "Authorization header required",
			})
		}

		// Check Bearer prefix
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"message": "Invalid authorization format",
			})
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or expired token",
			})
		}

		// Set user info in context
		c.Locals("userID", claims.UserID)
		c.Locals("userEmail", claims.Email)
		c.Locals("userName", claims.Name)

		return c.Next()
	}
}
