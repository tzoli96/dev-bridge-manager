// backend/internal/handlers/client_health_handler.go
package handlers

import (
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type ClientHealthHandler struct{}

func NewClientHealthHandler() *ClientHealthHandler {
	return &ClientHealthHandler{}
}

type ClientHealthListResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message,omitempty"`
	Clients []services.ClientHealth `json:"clients,omitempty"`
}

// List - GET /api/v1/admin/client-health - risk status (green/yellow/red)
// for every client, combining stalled kanban tasks and overdue invoices
// (see services.ListClientHealth). super_admin only.
func (h *ClientHealthHandler) List(c *fiber.Ctx) error {
	health, err := services.ListClientHealth()
	if err != nil {
		return c.Status(500).JSON(ClientHealthListResponse{Success: false, Message: "Error computing client health"})
	}

	return c.JSON(ClientHealthListResponse{Success: true, Clients: health})
}
