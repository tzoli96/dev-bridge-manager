package handlers

import (
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type RoleHandler struct {
	permissionService *services.PermissionService
}

type CreateRoleRequest struct {
	Name          string `json:"name" validate:"required"`
	DisplayName   string `json:"display_name" validate:"required"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permission_ids"`
}

type UpdateRoleRequest struct {
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	IsActive      *bool  `json:"is_active"`
	PermissionIDs []uint `json:"permission_ids"`
}

type RoleResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Role    *models.Role  `json:"role,omitempty"`
	Roles   []models.Role `json:"roles,omitempty"`
}

func NewRoleHandler() *RoleHandler {
	return &RoleHandler{
		permissionService: services.NewPermissionService(),
	}
}

// GetAllRoles returns all roles
func (h *RoleHandler) GetAllRoles(c *fiber.Ctx) error {
	roles, err := h.permissionService.GetAllRoles()
	if err != nil {
		return c.Status(500).JSON(RoleResponse{
			Success: false,
			Message: "Failed to fetch roles",
		})
	}

	return c.JSON(RoleResponse{
		Success: true,
		Message: "Roles retrieved successfully",
		Roles:   roles,
	})
}

// CreateRole creates a new role
func (h *RoleHandler) CreateRole(c *fiber.Ctx) error {
	var req CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(RoleResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	role := models.Role{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		IsActive:    true,
	}

	if err := h.permissionService.CreateRole(&role); err != nil {
		return c.Status(500).JSON(RoleResponse{
			Success: false,
			Message: "Failed to create role",
		})
	}

	// Assign permissions if provided
	if len(req.PermissionIDs) > 0 {
		if err := h.permissionService.AssignPermissionsToRole(role.ID, req.PermissionIDs); err != nil {
			return c.Status(500).JSON(RoleResponse{
				Success: false,
				Message: "Role created but failed to assign permissions",
			})
		}
	}

	return c.Status(201).JSON(RoleResponse{
		Success: true,
		Message: "Role created successfully",
		Role:    &role,
	})
}

// UpdateRole updates a role
func (h *RoleHandler) UpdateRole(c *fiber.Ctx) error {
	roleID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(RoleResponse{
			Success: false,
			Message: "Invalid role ID",
		})
	}

	var req UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(RoleResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	updates := make(map[string]interface{})
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := h.permissionService.UpdateRole(uint(roleID), updates); err != nil {
		return c.Status(500).JSON(RoleResponse{
			Success: false,
			Message: "Failed to update role",
		})
	}

	// Update permissions if provided
	if len(req.PermissionIDs) > 0 {
		if err := h.permissionService.AssignPermissionsToRole(uint(roleID), req.PermissionIDs); err != nil {
			return c.Status(500).JSON(RoleResponse{
				Success: false,
				Message: "Role updated but failed to assign permissions",
			})
		}
	}

	return c.JSON(RoleResponse{
		Success: true,
		Message: "Role updated successfully",
	})
}
