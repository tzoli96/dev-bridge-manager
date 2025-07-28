package handlers

import (
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type PermissionHandler struct {
	permissionService *services.PermissionService
}

type PermissionResponse struct {
	Success     bool                `json:"success"`
	Message     string              `json:"message"`
	Permission  *models.Permission  `json:"permission,omitempty"`
	Permissions []models.Permission `json:"permissions,omitempty"`
}

func NewPermissionHandler() *PermissionHandler {
	return &PermissionHandler{
		permissionService: services.NewPermissionService(),
	}
}

// GetAllPermissions returns all permissions
func (h *PermissionHandler) GetAllPermissions(c *fiber.Ctx) error {
	permissions, err := h.permissionService.GetAllPermissions()
	if err != nil {
		return c.Status(500).JSON(PermissionResponse{
			Success: false,
			Message: "Failed to fetch permissions",
		})
	}

	return c.JSON(PermissionResponse{
		Success:     true,
		Message:     "Permissions retrieved successfully",
		Permissions: permissions,
	})
}

// GetUserPermissions returns current user's permissions
func (h *PermissionHandler) GetUserPermissions(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	user, err := h.permissionService.GetUserWithPermissions(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get user permissions",
		})
	}

	return c.JSON(fiber.Map{
		"success":     true,
		"message":     "User permissions retrieved",
		"permissions": user.GetPermissions(),
		"role":        user.Role.Name,
	})
}
