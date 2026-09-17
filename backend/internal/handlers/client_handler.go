package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type ClientHandler struct {
	permissionService *services.PermissionService
}

func NewClientHandler() *ClientHandler {
	return &ClientHandler{
		permissionService: services.NewPermissionService(),
	}
}

// validateClientCreateRequest - egyszerű validáció validator csomag nélkül
func (h *ClientHandler) validateClientCreateRequest(req *models.ClientCreateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fiber.NewError(400, "Client name is required")
	}
	if len(req.Name) > 255 {
		return fiber.NewError(400, "Client name must be less than 255 characters")
	}
	if req.Type != "" && req.Type != "company" && req.Type != "individual" {
		return fiber.NewError(400, "Type must be one of: company, individual")
	}
	if req.Type != "individual" && strings.TrimSpace(req.TaxNumber) == "" {
		return fiber.NewError(400, "Tax number is required for company clients")
	}
	return nil
}

// validateClientUpdateRequest - egyszerű validáció validator csomag nélkül
func (h *ClientHandler) validateClientUpdateRequest(req *models.ClientUpdateRequest) error {
	if req.Name != "" && len(req.Name) > 255 {
		return fiber.NewError(400, "Client name must be less than 255 characters")
	}
	if req.Type != "" && req.Type != "company" && req.Type != "individual" {
		return fiber.NewError(400, "Type must be one of: company, individual")
	}
	return nil
}

// checkClientPermission - jogosultság ellenőrzése egy adott clients.* jogra, admin/super_admin fallback-kel
func (h *ClientHandler) checkClientPermission(userID uint, permission string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, permission)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

func toClientResponse(client models.Client) models.ClientResponse {
	return models.ClientResponse{
		ID:                client.ID,
		Type:              client.Type,
		Name:              client.Name,
		TaxNumber:         client.TaxNumber,
		EUVatNumber:       client.EUVatNumber,
		CompanyRegNumber:  client.CompanyRegNumber,
		BillingZip:        client.BillingZip,
		BillingCity:       client.BillingCity,
		BillingAddress:    client.BillingAddress,
		BankAccountNumber: client.BankAccountNumber,
		Email:             client.Email,
		Phone:             client.Phone,
		Notes:             client.Notes,
		IsActive:          client.IsActive,
		CreatedBy:         client.CreatedBy,
		CreatedByName:     client.CreatedByName,
		CreatedAt:         client.CreatedAt,
		UpdatedAt:         client.UpdatedAt,
	}
}

// GetAllClients - GET /api/v1/clients
// Mindenki láthatja aki be van jelentkezve
func (h *ClientHandler) GetAllClients(c *fiber.Ctx) error {
	var clients []models.Client

	err := database.GetDB().Table("clients").
		Select("clients.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON clients.created_by = users.id").
		Order("clients.name ASC").
		Scan(&clients).Error

	if err != nil {
		return c.Status(500).JSON(models.ClientListResponse{
			Success: false,
			Message: "Error fetching clients",
		})
	}

	response := make([]models.ClientResponse, 0, len(clients))
	for _, client := range clients {
		response = append(response, toClientResponse(client))
	}

	return c.JSON(models.ClientListResponse{
		Success: true,
		Message: "Clients retrieved successfully",
		Clients: response,
		Count:   len(response),
	})
}

// GetClient - GET /api/v1/clients/:id
func (h *ClientHandler) GetClient(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ClientListResponse{
			Success: false,
			Message: "Invalid client ID",
		})
	}

	var client models.Client
	err = database.GetDB().Table("clients").
		Select("clients.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON clients.created_by = users.id").
		Where("clients.id = ?", id).
		First(&client).Error

	if err != nil {
		return c.Status(404).JSON(models.ClientListResponse{
			Success: false,
			Message: "Client not found",
		})
	}

	return c.JSON(models.ClientListResponse{
		Success: true,
		Message: "Client retrieved successfully",
		Client:  ptrClientResponse(toClientResponse(client)),
	})
}

func ptrClientResponse(r models.ClientResponse) *models.ClientResponse {
	return &r
}

// CreateClient - POST /api/v1/clients
// Csak admin hozhat létre ügyfelet
func (h *ClientHandler) CreateClient(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkClientPermission(currentUserID, "clients.create"); err != nil {
		return c.Status(err.(*fiber.Error).Code).JSON(models.ClientListResponse{
			Success: false,
			Message: err.(*fiber.Error).Message,
		})
	}

	var req models.ClientCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ClientListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if err := h.validateClientCreateRequest(&req); err != nil {
		return err
	}

	clientType := req.Type
	if clientType == "" {
		clientType = "company"
	}

	client := models.Client{
		Type:              clientType,
		Name:              req.Name,
		TaxNumber:         req.TaxNumber,
		EUVatNumber:       req.EUVatNumber,
		CompanyRegNumber:  req.CompanyRegNumber,
		BillingZip:        req.BillingZip,
		BillingCity:       req.BillingCity,
		BillingAddress:    req.BillingAddress,
		BankAccountNumber: req.BankAccountNumber,
		Email:             req.Email,
		Phone:             req.Phone,
		Notes:             req.Notes,
		CreatedBy:         currentUserID,
	}

	if err := database.GetDB().Create(&client).Error; err != nil {
		return c.Status(500).JSON(models.ClientListResponse{
			Success: false,
			Message: "Error creating client",
		})
	}

	var createdClient models.Client
	database.GetDB().Table("clients").
		Select("clients.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON clients.created_by = users.id").
		Where("clients.id = ?", client.ID).
		First(&createdClient)

	return c.Status(201).JSON(models.ClientListResponse{
		Success: true,
		Message: "Client created successfully",
		Client:  ptrClientResponse(toClientResponse(createdClient)),
	})
}

// UpdateClient - PUT /api/v1/clients/:id
// Csak admin frissítheti az ügyfelet
func (h *ClientHandler) UpdateClient(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkClientPermission(currentUserID, "clients.update"); err != nil {
		return c.Status(err.(*fiber.Error).Code).JSON(models.ClientListResponse{
			Success: false,
			Message: err.(*fiber.Error).Message,
		})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ClientListResponse{
			Success: false,
			Message: "Invalid client ID",
		})
	}

	var req models.ClientUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ClientListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if err := h.validateClientUpdateRequest(&req); err != nil {
		return err
	}

	var client models.Client
	if err := database.GetDB().First(&client, id).Error; err != nil {
		return c.Status(404).JSON(models.ClientListResponse{
			Success: false,
			Message: "Client not found",
		})
	}

	updates := map[string]interface{}{}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.TaxNumber != "" {
		updates["tax_number"] = req.TaxNumber
	}
	if req.EUVatNumber != "" {
		updates["eu_vat_number"] = req.EUVatNumber
	}
	if req.CompanyRegNumber != "" {
		updates["company_reg_number"] = req.CompanyRegNumber
	}
	if req.BillingZip != "" {
		updates["billing_zip"] = req.BillingZip
	}
	if req.BillingCity != "" {
		updates["billing_city"] = req.BillingCity
	}
	if req.BillingAddress != "" {
		updates["billing_address"] = req.BillingAddress
	}
	if req.BankAccountNumber != "" {
		updates["bank_account_number"] = req.BankAccountNumber
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := database.GetDB().Model(&client).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(models.ClientListResponse{
				Success: false,
				Message: "Error updating client",
			})
		}
	}

	var updatedClient models.Client
	database.GetDB().Table("clients").
		Select("clients.*, users.name as created_by_name").
		Joins("LEFT JOIN users ON clients.created_by = users.id").
		Where("clients.id = ?", id).
		First(&updatedClient)

	return c.JSON(models.ClientListResponse{
		Success: true,
		Message: "Client updated successfully",
		Client:  ptrClientResponse(toClientResponse(updatedClient)),
	})
}

// DeleteClient - DELETE /api/v1/clients/:id
// Csak admin törölhet ügyfelet
func (h *ClientHandler) DeleteClient(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkClientPermission(currentUserID, "clients.delete"); err != nil {
		return c.Status(err.(*fiber.Error).Code).JSON(models.ClientListResponse{
			Success: false,
			Message: err.(*fiber.Error).Message,
		})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.ClientListResponse{
			Success: false,
			Message: "Invalid client ID",
		})
	}

	var client models.Client
	if err := database.GetDB().First(&client, id).Error; err != nil {
		return c.Status(404).JSON(models.ClientListResponse{
			Success: false,
			Message: "Client not found",
		})
	}

	if err := database.GetDB().Delete(&client).Error; err != nil {
		return c.Status(500).JSON(models.ClientListResponse{
			Success: false,
			Message: "Error deleting client",
		})
	}

	return c.JSON(models.ClientListResponse{
		Success: true,
		Message: "Client deleted successfully",
	})
}
