// backend/internal/handlers/project_password_handler.go
package handlers

import (
	"log"
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type ProjectPasswordHandler struct{}

func NewProjectPasswordHandler() *ProjectPasswordHandler {
	return &ProjectPasswordHandler{}
}

func buildPasswordDTO(p models.ProjectPassword, plaintext string) models.ProjectPasswordDTO {
	var createdByName string
	var user models.User
	if err := database.GetDB().Select("name").First(&user, p.CreatedBy).Error; err == nil {
		createdByName = user.Name
	}
	return models.ProjectPasswordDTO{
		ID:            p.ID,
		ProjectID:     p.ProjectID,
		Title:         p.Title,
		Username:      p.Username,
		Password:      plaintext,
		URL:           p.URL,
		Notes:         p.Notes,
		CreatedBy:     p.CreatedBy,
		CreatedByName: createdByName,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

// GetProjectPasswords - GET /api/v1/projects/:id/passwords
func (h *ProjectPasswordHandler) GetProjectPasswords(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var passwords []models.ProjectPassword
	if err := database.GetDB().Where("project_id = ?", projectID).Order("created_at DESC").Find(&passwords).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading passwords"})
	}

	dtos := make([]models.ProjectPasswordDTO, 0, len(passwords))
	for _, p := range passwords {
		plaintext, err := services.Decrypt(p.EncryptedPassword)
		if err != nil {
			log.Printf("project password %d: failed to decrypt: %v", p.ID, err)
			continue
		}
		dtos = append(dtos, buildPasswordDTO(p, plaintext))
	}

	return c.JSON(dtos)
}

// CreateProjectPassword - POST /api/v1/projects/:id/passwords
func (h *ProjectPasswordHandler) CreateProjectPassword(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.ProjectPasswordCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title and password are required"})
	}

	encrypted, err := services.Encrypt(req.Password)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error encrypting password"})
	}

	userID := currentUserID(c)
	password := models.ProjectPassword{
		ProjectID:         uint(projectID),
		Title:             req.Title,
		Username:          req.Username,
		EncryptedPassword: encrypted,
		URL:               req.URL,
		Notes:             req.Notes,
		CreatedBy:         userID,
		UpdatedBy:         userID,
	}
	if err := database.GetDB().Create(&password).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating password"})
	}

	return c.Status(201).JSON(buildPasswordDTO(password, req.Password))
}

// UpdateProjectPassword - PUT /api/v1/projects/:id/passwords/:passwordId
func (h *ProjectPasswordHandler) UpdateProjectPassword(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	passwordID, err := strconv.Atoi(c.Params("passwordId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid password ID"})
	}

	var password models.ProjectPassword
	if err := database.GetDB().Where("id = ? AND project_id = ?", passwordID, projectID).First(&password).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Password not found"})
	}

	var req models.ProjectPasswordUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title and password are required"})
	}

	encrypted, err := services.Encrypt(req.Password)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error encrypting password"})
	}

	password.Title = req.Title
	password.Username = req.Username
	password.EncryptedPassword = encrypted
	password.URL = req.URL
	password.Notes = req.Notes
	password.UpdatedBy = currentUserID(c)

	if err := database.GetDB().Save(&password).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating password"})
	}

	return c.JSON(buildPasswordDTO(password, req.Password))
}

// DeleteProjectPassword - DELETE /api/v1/projects/:id/passwords/:passwordId
func (h *ProjectPasswordHandler) DeleteProjectPassword(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	passwordID, err := strconv.Atoi(c.Params("passwordId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid password ID"})
	}

	result := database.GetDB().Where("id = ? AND project_id = ?", passwordID, projectID).Delete(&models.ProjectPassword{})
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting password"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Password not found"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Password deleted successfully"})
}
