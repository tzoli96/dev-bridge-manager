package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type ProjectHandler struct {
	permissionService *services.PermissionService
}

func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{
		permissionService: services.NewPermissionService(),
	}
}

// validateProjectCreateRequest - egyszerű validáció validator csomag nélkül
func (h *ProjectHandler) validateProjectCreateRequest(req *models.ProjectCreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "Project name is required")
	}
	if len(req.Name) > 255 {
		return fiber.NewError(400, "Project name must be less than 255 characters")
	}
	if req.Status != "" && req.Status != "active" && req.Status != "completed" && req.Status != "on-hold" && req.Status != "cancelled" {
		return fiber.NewError(400, "Status must be one of: active, completed, on-hold, cancelled")
	}
	return nil
}

// validateProjectUpdateRequest - egyszerű validáció validator csomag nélkül
func (h *ProjectHandler) validateProjectUpdateRequest(req *models.ProjectUpdateRequest) error {
	if req.Name != "" && len(req.Name) > 255 {
		return fiber.NewError(400, "Project name must be less than 255 characters")
	}
	if req.Status != "" && req.Status != "active" && req.Status != "completed" && req.Status != "on-hold" && req.Status != "cancelled" {
		return fiber.NewError(400, "Status must be one of: active, completed, on-hold, cancelled")
	}
	return nil
}

// GetAllProjects - GET /api/v1/projects
// Mindenki láthatja a projekteket aki be van jelentkezve
func (h *ProjectHandler) GetAllProjects(c *fiber.Ctx) error {
	var projects []models.Project

	// Join-nel lekérjük a létrehozó nevét is
	err := database.GetDB().Table("projects").
		Select("projects.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON projects.created_by = users.id").
		Order("projects.created_at DESC").
		Scan(&projects).Error

	if err != nil {
		return c.Status(500).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Error fetching projects",
		})
	}

	// Convert to response format
	var response []models.ProjectResponse
	for _, project := range projects {
		response = append(response, models.ProjectResponse{
			ID:            project.ID,
			Name:          project.Name,
			Description:   project.Description,
			Status:        project.Status,
			CreatedBy:     project.CreatedBy,
			CreatedByName: project.CreatedByName,
			CreatedAt:     project.CreatedAt,
			UpdatedAt:     project.UpdatedAt,
		})
	}

	return c.JSON(models.ProjectListResponse{
		Success:  true,
		Message:  "Projects retrieved successfully",
		Projects: response,
		Count:    len(response),
	})
}

// GetProject - GET /api/v1/projects/:id
// Mindenki láthatja egy adott projektet
func (h *ProjectHandler) GetProject(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	var project models.Project
	err = database.GetDB().Table("projects").
		Select("projects.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON projects.created_by = users.id").
		Where("projects.id = ?", id).
		First(&project).Error

	if err != nil {
		return c.Status(404).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	response := models.ProjectResponse{
		ID:            project.ID,
		Name:          project.Name,
		Description:   project.Description,
		Status:        project.Status,
		CreatedBy:     project.CreatedBy,
		CreatedByName: project.CreatedByName,
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.UpdatedAt,
	}

	return c.JSON(models.ProjectListResponse{
		Success: true,
		Message: "Project retrieved successfully",
		Project: &response,
	})
}

// CreateProject - POST /api/v1/projects
// Csak admin hozhat létre projektet
func (h *ProjectHandler) CreateProject(c *fiber.Ctx) error {
	// Admin jogosultság ellenőrzése
	currentUserID := c.Locals("userID").(uint)

	// Ellenőrizzük, hogy admin-e
	hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "projects.create")
	if err != nil || !hasPermission {
		// Alternatívaként role alapján is ellenőrizhetjük
		user, err := h.permissionService.GetUserWithPermissions(currentUserID)
		if err != nil {
			return c.Status(500).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Error checking permissions",
			})
		}

		// Admin vagy super_admin szerepkör ellenőrzése
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" {
			return c.Status(403).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Only administrators can create projects",
			})
		}
	}

	var req models.ProjectCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Validáció
	if err := h.validateProjectCreateRequest(&req); err != nil {
		return err
	}

	// Default status beállítása
	if req.Status == "" {
		req.Status = "active"
	}

	// Projekt létrehozása
	project := models.Project{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   currentUserID,
	}

	if err := database.GetDB().Create(&project).Error; err != nil {
		return c.Status(500).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Error creating project",
		})
	}

	// Visszatöltjük a létrehozó nevével
	var createdProject models.Project
	database.GetDB().Table("projects").
		Select("projects.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON projects.created_by = users.id").
		Where("projects.id = ?", project.ID).
		First(&createdProject)

	response := models.ProjectResponse{
		ID:            createdProject.ID,
		Name:          createdProject.Name,
		Description:   createdProject.Description,
		Status:        createdProject.Status,
		CreatedBy:     createdProject.CreatedBy,
		CreatedByName: createdProject.CreatedByName,
		CreatedAt:     createdProject.CreatedAt,
		UpdatedAt:     createdProject.UpdatedAt,
	}

	return c.Status(201).JSON(models.ProjectListResponse{
		Success: true,
		Message: "Project created successfully",
		Project: &response,
	})
}

// UpdateProject - PUT /api/v1/projects/:id
// Csak admin frissíthet projektet
func (h *ProjectHandler) UpdateProject(c *fiber.Ctx) error {
	// Admin jogosultság ellenőrzése
	currentUserID := c.Locals("userID").(uint)

	// Ellenőrizzük, hogy admin-e
	hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "projects.update")
	if err != nil || !hasPermission {
		// Alternatívaként role alapján is ellenőrizhetjük
		user, err := h.permissionService.GetUserWithPermissions(currentUserID)
		if err != nil {
			return c.Status(500).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Error checking permissions",
			})
		}

		// Admin vagy super_admin szerepkör ellenőrzése
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" {
			return c.Status(403).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Only administrators can update projects",
			})
		}
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	var req models.ProjectUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Validáció
	if err := h.validateProjectUpdateRequest(&req); err != nil {
		return err
	}

	// Projekt keresése
	var project models.Project
	if err := database.GetDB().First(&project, id).Error; err != nil {
		return c.Status(404).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	// Csak a megadott mezők frissítése
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	if len(updates) > 0 {
		if err := database.GetDB().Model(&project).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Error updating project",
			})
		}
	}

	// Frissített projekt visszatöltése
	var updatedProject models.Project
	database.GetDB().Table("projects").
		Select("projects.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON projects.created_by = users.id").
		Where("projects.id = ?", id).
		First(&updatedProject)

	response := models.ProjectResponse{
		ID:            updatedProject.ID,
		Name:          updatedProject.Name,
		Description:   updatedProject.Description,
		Status:        updatedProject.Status,
		CreatedBy:     updatedProject.CreatedBy,
		CreatedByName: updatedProject.CreatedByName,
		CreatedAt:     updatedProject.CreatedAt,
		UpdatedAt:     updatedProject.UpdatedAt,
	}

	return c.JSON(models.ProjectListResponse{
		Success: true,
		Message: "Project updated successfully",
		Project: &response,
	})
}

// DeleteProject - DELETE /api/v1/projects/:id
// Csak admin törölhet projektet
func (h *ProjectHandler) DeleteProject(c *fiber.Ctx) error {
	// Admin jogosultság ellenőrzése
	currentUserID := c.Locals("userID").(uint)

	// Ellenőrizzük, hogy admin-e
	hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "projects.delete")
	if err != nil || !hasPermission {
		// Alternatívaként role alapján is ellenőrizhetjük
		user, err := h.permissionService.GetUserWithPermissions(currentUserID)
		if err != nil {
			return c.Status(500).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Error checking permissions",
			})
		}

		// Admin vagy super_admin szerepkör ellenőrzése
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" {
			return c.Status(403).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Only administrators can delete projects",
			})
		}
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Invalid project ID",
		})
	}

	// Projekt keresése
	var project models.Project
	if err := database.GetDB().First(&project, id).Error; err != nil {
		return c.Status(404).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Project not found",
		})
	}

	// Projekt törlése
	if err := database.GetDB().Delete(&project).Error; err != nil {
		return c.Status(500).JSON(models.ProjectListResponse{
			Success: false,
			Message: "Error deleting project",
		})
	}

	return c.JSON(models.ProjectListResponse{
		Success: true,
		Message: "Project deleted successfully",
	})
}
