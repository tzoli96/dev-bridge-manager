// backend/internal/handlers/project_environment_handler.go
package handlers

import (
	"strconv"
	"strings"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

type ProjectEnvironmentHandler struct{}

func NewProjectEnvironmentHandler() *ProjectEnvironmentHandler {
	return &ProjectEnvironmentHandler{}
}

// validateEnvironmentRequest trims the request in place and returns a
// user-facing message for the first problem found, or "" when it is valid.
// URL and repo are optional, but when present the URL must be http(s) and
// the repo must be an https/ssh URL or scp-style (git@host:org/repo).
func validateEnvironmentRequest(req *models.ProjectEnvironmentRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	req.GitRepoURL = strings.TrimSpace(req.GitRepoURL)

	if req.Name == "" {
		return "Name is required"
	}
	if len(req.Name) > 255 {
		return "Name is too long"
	}
	if req.URL != "" && !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		return "URL must start with http:// or https://"
	}
	if req.GitRepoURL != "" &&
		!strings.HasPrefix(req.GitRepoURL, "https://") &&
		!strings.HasPrefix(req.GitRepoURL, "ssh://") &&
		!strings.HasPrefix(req.GitRepoURL, "git@") {
		return "Git repo must be an https://, ssh:// or git@ address"
	}
	if len(req.URL) > 500 || len(req.GitRepoURL) > 500 {
		return "URL is too long"
	}
	return ""
}

// GetProjectEnvironments - GET /api/v1/projects/:id/environments
func (h *ProjectEnvironmentHandler) GetProjectEnvironments(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var environments []models.ProjectEnvironment
	if err := database.GetDB().Where("project_id = ?", projectID).Order("created_at ASC, id ASC").Find(&environments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading environments"})
	}

	return c.JSON(environments)
}

// CreateProjectEnvironment - POST /api/v1/projects/:id/environments
func (h *ProjectEnvironmentHandler) CreateProjectEnvironment(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.ProjectEnvironmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateEnvironmentRequest(&req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	environment := models.ProjectEnvironment{
		ProjectID:  uint(projectID),
		Name:       req.Name,
		URL:        req.URL,
		GitRepoURL: req.GitRepoURL,
	}
	if err := database.GetDB().Create(&environment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating environment"})
	}

	return c.Status(201).JSON(environment)
}

// UpdateProjectEnvironment - PUT /api/v1/projects/:id/environments/:environmentId
func (h *ProjectEnvironmentHandler) UpdateProjectEnvironment(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	environmentID, err := strconv.Atoi(c.Params("environmentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid environment ID"})
	}

	var environment models.ProjectEnvironment
	if err := database.GetDB().Where("id = ? AND project_id = ?", environmentID, projectID).First(&environment).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Environment not found"})
	}

	var req models.ProjectEnvironmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateEnvironmentRequest(&req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	environment.Name = req.Name
	environment.URL = req.URL
	environment.GitRepoURL = req.GitRepoURL

	if err := database.GetDB().Save(&environment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating environment"})
	}

	return c.JSON(environment)
}

// DeleteProjectEnvironment - DELETE /api/v1/projects/:id/environments/:environmentId
func (h *ProjectEnvironmentHandler) DeleteProjectEnvironment(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	environmentID, err := strconv.Atoi(c.Params("environmentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid environment ID"})
	}

	result := database.GetDB().Where("id = ? AND project_id = ?", environmentID, projectID).Delete(&models.ProjectEnvironment{})
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting environment"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Environment not found"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Environment deleted successfully"})
}
