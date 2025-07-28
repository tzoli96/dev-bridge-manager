// handlers/project_assignment_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type ProjectAssignmentHandler struct {
	permissionService *services.PermissionService
}

func NewProjectAssignmentHandler() *ProjectAssignmentHandler {
	return &ProjectAssignmentHandler{
		permissionService: services.NewPermissionService(),
	}
}

// validateAssignmentCreateRequest - egyszerű validáció
func (h *ProjectAssignmentHandler) validateAssignmentCreateRequest(req *models.ProjectAssignmentCreateRequest) error {
	if req.UserID == 0 {
		return fiber.NewError(400, "User ID is required")
	}
	if req.Role != "" && req.Role != "owner" && req.Role != "manager" && req.Role != "member" && req.Role != "viewer" {
		return fiber.NewError(400, "Role must be one of: owner, manager, member, viewer")
	}
	return nil
}

// validateAssignmentUpdateRequest - egyszerű validáció
func (h *ProjectAssignmentHandler) validateAssignmentUpdateRequest(req *models.ProjectAssignmentUpdateRequest) error {
	if strings.TrimSpace(req.Role) == "" {
		return fiber.NewError(400, "Role is required")
	}
	if req.Role != "owner" && req.Role != "manager" && req.Role != "member" && req.Role != "viewer" {
		return fiber.NewError(400, "Role must be one of: owner, manager, member, viewer")
	}
	return nil
}

// checkProjectAccess - ellenőrzi hogy a user hozzáfér-e a projekthez
func (h *ProjectAssignmentHandler) checkProjectAccess(userID uint, action string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, action)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}

		// Admin, super_admin és manager jogok ellenőrzése
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" && user.Role.Name != "manager" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

// GetProjectAssignments - GET /api/v1/projects/:id/assignments
func (h *ProjectAssignmentHandler) GetProjectAssignments(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	// Projekt létezésének ellenőrzése
	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	var assignments []models.ProjectAssignmentResponse
	err = database.GetDB().Table("project_assignments").
		Select(`project_assignments.*, 
				users.name as user_name, 
				users.email as user_email,
				assigned_by_user.name as assigned_by_name`).
		Joins("LEFT JOIN users ON project_assignments.user_id = users.id").
		Joins("LEFT JOIN users assigned_by_user ON project_assignments.assigned_by = assigned_by_user.id").
		Where("project_assignments.project_id = ? AND project_assignments.is_active = ?", projectID, true).
		Order("project_assignments.assigned_at DESC").
		Scan(&assignments).Error

	if err != nil {
		return c.Status(500).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Error fetching project assignments",
		})
	}

	return c.JSON(models.ProjectAssignmentListResponse{
		Success:     true,
		Message:     "Project assignments retrieved successfully",
		Assignments: assignments,
		Count:       len(assignments),
	})
}

// AssignUserToProject - POST /api/v1/projects/:id/assignments
func (h *ProjectAssignmentHandler) AssignUserToProject(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	// Jogosultság ellenőrzése
	if err := h.checkProjectAccess(currentUserID, "project_assignments.create"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	// Projekt létezésének ellenőrzése
	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	var req models.ProjectAssignmentCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Validáció
	if err := h.validateAssignmentCreateRequest(&req); err != nil {
		return err
	}

	// User létezésének ellenőrzése
	var user models.User
	if err := database.GetDB().First(&user, req.UserID).Error; err != nil {
		return c.Status(404).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	// Ellenőrizzük hogy nincs-e már hozzárendelve
	var existingAssignment models.ProjectAssignment
	err = database.GetDB().Where("project_id = ? AND user_id = ?", projectID, req.UserID).First(&existingAssignment).Error
	if err == nil {
		if existingAssignment.IsActive {
			return c.Status(409).JSON(models.ProjectAssignmentListResponse{
				Success: false,
				Message: "User is already assigned to this project",
			})
		}
		// Ha inactive, akkor aktiválni
		existingAssignment.IsActive = true
		existingAssignment.Role = req.Role
		if req.Role == "" {
			existingAssignment.Role = "member"
		}
		existingAssignment.AssignedBy = currentUserID

		if err := database.GetDB().Save(&existingAssignment).Error; err != nil {
			return c.Status(500).JSON(models.ProjectAssignmentListResponse{
				Success: false,
				Message: "Error reactivating assignment",
			})
		}
	} else {
		// Default role beállítása
		if req.Role == "" {
			req.Role = "member"
		}

		// Új hozzárendelés létrehozása
		assignment := models.ProjectAssignment{
			ProjectID:  uint(projectID),
			UserID:     req.UserID,
			Role:       req.Role,
			AssignedBy: currentUserID,
			IsActive:   true,
		}

		if err := database.GetDB().Create(&assignment).Error; err != nil {
			return c.Status(500).JSON(models.ProjectAssignmentListResponse{
				Success: false,
				Message: "Error creating assignment",
			})
		}
	}

	// Visszatöltjük a létrehozott/módosított assignment-et
	var responseAssignment models.ProjectAssignmentResponse
	database.GetDB().Table("project_assignments").
		Select(`project_assignments.*, 
				users.name as user_name, 
				users.email as user_email,
				assigned_by_user.name as assigned_by_name`).
		Joins("LEFT JOIN users ON project_assignments.user_id = users.id").
		Joins("LEFT JOIN users assigned_by_user ON project_assignments.assigned_by = assigned_by_user.id").
		Where("project_assignments.project_id = ? AND project_assignments.user_id = ?", projectID, req.UserID).
		Scan(&responseAssignment)

	return c.Status(201).JSON(models.ProjectAssignmentListResponse{
		Success:    true,
		Message:    "User assigned to project successfully",
		Assignment: &responseAssignment,
	})
}

// UpdateProjectAssignment - PUT /api/v1/projects/:id/assignments/:user_id
func (h *ProjectAssignmentHandler) UpdateProjectAssignment(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	// Jogosultság ellenőrzése
	if err := h.checkProjectAccess(currentUserID, "project_assignments.update"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	var req models.ProjectAssignmentUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Validáció
	if err := h.validateAssignmentUpdateRequest(&req); err != nil {
		return err
	}

	// Assignment keresése
	var assignment models.ProjectAssignment
	err = database.GetDB().Where("project_id = ? AND user_id = ? AND is_active = ?", projectID, userID, true).First(&assignment).Error
	if err != nil {
		return c.Status(404).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Assignment not found",
		})
	}

	// Role frissítése
	assignment.Role = req.Role
	if err := database.GetDB().Save(&assignment).Error; err != nil {
		return c.Status(500).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Error updating assignment",
		})
	}

	// Frissített assignment visszatöltése
	var responseAssignment models.ProjectAssignmentResponse
	database.GetDB().Table("project_assignments").
		Select(`project_assignments.*, 
				users.name as user_name, 
				users.email as user_email,
				assigned_by_user.name as assigned_by_name`).
		Joins("LEFT JOIN users ON project_assignments.user_id = users.id").
		Joins("LEFT JOIN users assigned_by_user ON project_assignments.assigned_by = assigned_by_user.id").
		Where("project_assignments.project_id = ? AND project_assignments.user_id = ?", projectID, userID).
		Scan(&responseAssignment)

	return c.JSON(models.ProjectAssignmentListResponse{
		Success:    true,
		Message:    "Assignment updated successfully",
		Assignment: &responseAssignment,
	})
}

// RemoveUserFromProject - DELETE /api/v1/projects/:id/assignments/:user_id
func (h *ProjectAssignmentHandler) RemoveUserFromProject(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	// Jogosultság ellenőrzése
	if err := h.checkProjectAccess(currentUserID, "project_assignments.delete"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	// Assignment keresése
	var assignment models.ProjectAssignment
	err = database.GetDB().Where("project_id = ? AND user_id = ? AND is_active = ?", projectID, userID, true).First(&assignment).Error
	if err != nil {
		return c.Status(404).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Assignment not found",
		})
	}

	// Soft delete - is_active = false
	assignment.IsActive = false
	if err := database.GetDB().Save(&assignment).Error; err != nil {
		return c.Status(500).JSON(models.ProjectAssignmentListResponse{
			Success: false,
			Message: "Error removing assignment",
		})
	}

	return c.JSON(models.ProjectAssignmentListResponse{
		Success: true,
		Message: "User removed from project successfully",
	})
}
