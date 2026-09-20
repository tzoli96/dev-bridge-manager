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
	if err := validateProjectPricing(req.PricingType, req.HourlyRate, req.FixedPrice); err != nil {
		return err
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
	if err := validateProjectPricing(req.PricingType, req.HourlyRate, req.FixedPrice); err != nil {
		return err
	}
	return nil
}

// validateProjectPricing - óradíjas vagy fix díjas megállapodás ellenőrzése
func validateProjectPricing(pricingType string, hourlyRate, fixedPrice *float64) error {
	if pricingType == "" {
		return nil
	}
	if pricingType != "hourly" && pricingType != "fixed" {
		return fiber.NewError(400, "Pricing type must be one of: hourly, fixed")
	}
	if pricingType == "hourly" && (hourlyRate == nil || *hourlyRate <= 0) {
		return fiber.NewError(400, "Hourly rate is required and must be greater than 0 for hourly pricing")
	}
	if pricingType == "fixed" && (fixedPrice == nil || *fixedPrice <= 0) {
		return fiber.NewError(400, "Fixed price is required and must be greater than 0 for fixed pricing")
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
			ID:                     project.ID,
			Name:                   project.Name,
			Description:            project.Description,
			Status:                 project.Status,
			PricingType:            project.PricingType,
			HourlyRate:             project.HourlyRate,
			FixedPrice:             project.FixedPrice,
			AutoInvoiceEnabled:     project.AutoInvoiceEnabled,
			AutoInvoiceClientID:    project.AutoInvoiceClientID,
			AutoInvoiceAutoApprove: project.AutoInvoiceAutoApprove,
			CreatedBy:              project.CreatedBy,
			CreatedByName:          project.CreatedByName,
			CreatedAt:              project.CreatedAt,
			UpdatedAt:              project.UpdatedAt,
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
		ID:                     project.ID,
		Name:                   project.Name,
		Description:            project.Description,
		Status:                 project.Status,
		PricingType:            project.PricingType,
		HourlyRate:             project.HourlyRate,
		FixedPrice:             project.FixedPrice,
		AutoInvoiceEnabled:     project.AutoInvoiceEnabled,
		AutoInvoiceClientID:    project.AutoInvoiceClientID,
		AutoInvoiceAutoApprove: project.AutoInvoiceAutoApprove,
		CreatedBy:              project.CreatedBy,
		CreatedByName:          project.CreatedByName,
		CreatedAt:              project.CreatedAt,
		UpdatedAt:              project.UpdatedAt,
		Clients:                fetchProjectClients(uint(id)),
	}

	return c.JSON(models.ProjectListResponse{
		Success: true,
		Message: "Project retrieved successfully",
		Project: &response,
	})
}

// fetchProjectClients - egy projekthez rendelt ügyfelek lekérése
func fetchProjectClients(projectID uint) []models.ProjectClientResponse {
	var clients []models.ProjectClientResponse
	database.GetDB().Table("project_clients").
		Select("project_clients.id, project_clients.project_id, project_clients.client_id, clients.name as client_name, clients.type as client_type, project_clients.assigned_at, project_clients.assigned_by, users.name as assigned_by_name").
		Joins("LEFT JOIN clients ON project_clients.client_id = clients.id").
		Joins("LEFT JOIN users ON project_clients.assigned_by = users.id").
		Where("project_clients.project_id = ?", projectID).
		Scan(&clients)
	return clients
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
		PricingType: req.PricingType,
		HourlyRate:  req.HourlyRate,
		FixedPrice:  req.FixedPrice,
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
		ID:                     createdProject.ID,
		Name:                   createdProject.Name,
		Description:            createdProject.Description,
		Status:                 createdProject.Status,
		PricingType:            createdProject.PricingType,
		HourlyRate:             createdProject.HourlyRate,
		FixedPrice:             createdProject.FixedPrice,
		AutoInvoiceEnabled:     createdProject.AutoInvoiceEnabled,
		AutoInvoiceClientID:    createdProject.AutoInvoiceClientID,
		AutoInvoiceAutoApprove: createdProject.AutoInvoiceAutoApprove,
		CreatedBy:              createdProject.CreatedBy,
		CreatedByName:          createdProject.CreatedByName,
		CreatedAt:              createdProject.CreatedAt,
		UpdatedAt:              createdProject.UpdatedAt,
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
	if req.PricingType != "" {
		updates["pricing_type"] = req.PricingType
		updates["hourly_rate"] = req.HourlyRate
		updates["fixed_price"] = req.FixedPrice
	}

	if req.AutoInvoiceEnabled != nil || req.AutoInvoiceClientID != nil || req.AutoInvoiceAutoApprove != nil {
		effectivePricingType := project.PricingType
		if req.PricingType != "" {
			effectivePricingType = req.PricingType
		}
		effectiveEnabled := project.AutoInvoiceEnabled
		if req.AutoInvoiceEnabled != nil {
			effectiveEnabled = *req.AutoInvoiceEnabled
		}
		effectiveClientID := project.AutoInvoiceClientID
		if req.AutoInvoiceClientID != nil {
			effectiveClientID = req.AutoInvoiceClientID
		}
		effectiveAutoApprove := project.AutoInvoiceAutoApprove
		if req.AutoInvoiceAutoApprove != nil {
			effectiveAutoApprove = *req.AutoInvoiceAutoApprove
		}

		if effectiveEnabled {
			if effectivePricingType != "hourly" {
				return c.Status(400).JSON(models.ProjectListResponse{
					Success: false,
					Message: "Automatic invoicing requires hourly pricing",
				})
			}
			if effectiveClientID == nil {
				return c.Status(400).JSON(models.ProjectListResponse{
					Success: false,
					Message: "Automatic invoicing requires a client",
				})
			}
			var projectClient models.ProjectClient
			if err := database.GetDB().Where("project_id = ? AND client_id = ?", id, *effectiveClientID).First(&projectClient).Error; err != nil {
				return c.Status(400).JSON(models.ProjectListResponse{
					Success: false,
					Message: "Automatic invoicing client is not attached to this project",
				})
			}
		} else if effectiveAutoApprove {
			return c.Status(400).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Auto-approve requires automatic invoicing to be enabled",
			})
		}

		updates["auto_invoice_enabled"] = effectiveEnabled
		updates["auto_invoice_client_id"] = effectiveClientID
		updates["auto_invoice_auto_approve"] = effectiveAutoApprove
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
		ID:                     updatedProject.ID,
		Name:                   updatedProject.Name,
		Description:            updatedProject.Description,
		Status:                 updatedProject.Status,
		PricingType:            updatedProject.PricingType,
		HourlyRate:             updatedProject.HourlyRate,
		FixedPrice:             updatedProject.FixedPrice,
		AutoInvoiceEnabled:     updatedProject.AutoInvoiceEnabled,
		AutoInvoiceClientID:    updatedProject.AutoInvoiceClientID,
		AutoInvoiceAutoApprove: updatedProject.AutoInvoiceAutoApprove,
		CreatedBy:              updatedProject.CreatedBy,
		CreatedByName:          updatedProject.CreatedByName,
		CreatedAt:              updatedProject.CreatedAt,
		UpdatedAt:              updatedProject.UpdatedAt,
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
