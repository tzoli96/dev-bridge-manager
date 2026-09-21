// backend/internal/routes/email_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupEmailRoutes(api fiber.Router) {
	h := handlers.NewEmailHandler()
	emails := api.Group("/emails")
	emails.Use(middleware.JWTMiddleware())
	emails.Use(middleware.RequirePermission("gmail.manage"))

	emails.Get("/", h.ListEmails)
	emails.Get("/unread-count", h.GetUnreadCount)
	emails.Get("/:id", h.GetEmail)
	emails.Post("/:id/draft-reply", h.DraftReply)
	emails.Get("/:id/attachments/:attachmentId", h.GetAttachment)
	emails.Post("/send", h.SendEmail)
}
