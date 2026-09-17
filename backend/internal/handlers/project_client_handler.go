// handlers/project_client_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type ProjectClientHandler struct {
	permissionService *services.PermissionService
}

func NewProjectClientHandler() *ProjectClientHandler {
	return &ProjectClientHandler{
		permissionService: services.NewPermissionService(),
	}
}

// checkProjectClientAccess - ellenőrzi hogy a user hozzáfér-e a projekt-ügyfél kapcsolat kezeléséhez
func (h *ProjectClientHandler) checkProjectClientAccess(userID uint, action string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, action)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}

		if user.Role.Name != "admin" && user.Role.Name != "super_admin" && user.Role.Name != "manager" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

// GetProjectClients - GET /api/v1/projects/:id/clients
func (h *ProjectClientHandler) GetProjectClients(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	var projectClients []models.ProjectClientResponse
	err = database.GetDB().Table("project_clients").
		Select(`project_clients.id, project_clients.project_id, project_clients.client_id,
				clients.name as client_name, clients.type as client_type,
				project_clients.assigned_at, project_clients.assigned_by,
				users.name as assigned_by_name`).
		Joins("LEFT JOIN clients ON project_clients.client_id = clients.id").
		Joins("LEFT JOIN users ON project_clients.assigned_by = users.id").
		Where("project_clients.project_id = ?", projectID).
		Order("project_clients.assigned_at DESC").
		Scan(&projectClients).Error

	if err != nil {
		return c.Status(500).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Error fetching project clients",
		})
	}

	return c.JSON(models.ProjectClientListResponse{
		Success:        true,
		Message:        "Project clients retrieved successfully",
		ProjectClients: projectClients,
		Count:          len(projectClients),
	})
}

// AssignClientToProject - POST /api/v1/projects/:id/clients
func (h *ProjectClientHandler) AssignClientToProject(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkProjectClientAccess(currentUserID, "project_clients.create"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	var req models.ProjectClientCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if req.ClientID == 0 {
		return fiber.NewError(400, "Client ID is required")
	}

	var client models.Client
	if err := database.GetDB().First(&client, req.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Client not found",
		})
	}

	var existing models.ProjectClient
	err = database.GetDB().Where("project_id = ? AND client_id = ?", projectID, req.ClientID).First(&existing).Error
	if err == nil {
		return c.Status(409).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Client is already assigned to this project",
		})
	}

	projectClient := models.ProjectClient{
		ProjectID:  uint(projectID),
		ClientID:   req.ClientID,
		AssignedBy: currentUserID,
	}

	if err := database.GetDB().Create(&projectClient).Error; err != nil {
		return c.Status(500).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Error assigning client to project",
		})
	}

	var response models.ProjectClientResponse
	database.GetDB().Table("project_clients").
		Select(`project_clients.id, project_clients.project_id, project_clients.client_id,
				clients.name as client_name, clients.type as client_type,
				project_clients.assigned_at, project_clients.assigned_by,
				users.name as assigned_by_name`).
		Joins("LEFT JOIN clients ON project_clients.client_id = clients.id").
		Joins("LEFT JOIN users ON project_clients.assigned_by = users.id").
		Where("project_clients.id = ?", projectClient.ID).
		Scan(&response)

	return c.Status(201).JSON(models.ProjectClientListResponse{
		Success:       true,
		Message:       "Client assigned to project successfully",
		ProjectClient: &response,
	})
}

// RemoveClientFromProject - DELETE /api/v1/projects/:id/clients/:client_id
func (h *ProjectClientHandler) RemoveClientFromProject(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkProjectClientAccess(currentUserID, "project_clients.delete"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	clientID, err := strconv.Atoi(c.Params("client_id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Invalid client ID",
		})
	}

	var projectClient models.ProjectClient
	if err := database.GetDB().Where("project_id = ? AND client_id = ?", projectID, clientID).First(&projectClient).Error; err != nil {
		return c.Status(404).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Assignment not found",
		})
	}

	if err := database.GetDB().Delete(&projectClient).Error; err != nil {
		return c.Status(500).JSON(models.ProjectClientListResponse{
			Success: false,
			Message: "Error removing client from project",
		})
	}

	return c.JSON(models.ProjectClientListResponse{
		Success: true,
		Message: "Client removed from project successfully",
	})
}
