// backend/internal/handlers/jira_integration_handler.go
package handlers

import (
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type JiraIntegrationHandler struct {
	client services.JiraClient
}

func NewJiraIntegrationHandler() *JiraIntegrationHandler {
	return &JiraIntegrationHandler{client: services.NewRealJiraClient()}
}

func jiraStatusDTO(integration models.JiraBoardIntegration) models.JiraIntegrationStatusDTO {
	lastSyncAt := ""
	if integration.LastSyncAt != nil {
		lastSyncAt = integration.LastSyncAt.Format(time.RFC3339)
	}
	return models.JiraIntegrationStatusDTO{
		Connected:     true,
		BaseURL:       integration.BaseURL,
		Email:         integration.Email,
		ProjectKey:    integration.ProjectKey,
		LastSyncAt:    lastSyncAt,
		LastSyncError: integration.LastSyncError,
	}
}

// Connect - POST /api/v1/projects/:id/boards/:boardId/jira-integration
func (h *JiraIntegrationHandler) Connect(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var req models.ConnectJiraIntegrationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.BaseURL == "" || req.Email == "" || req.APIToken == "" || req.ProjectKey == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "baseUrl, email, apiToken, and projectKey are all required"})
	}

	if err := h.client.TestConnection(c.Context(), req.BaseURL, req.Email, req.APIToken); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Could not connect to Jira with the given credentials: " + err.Error()})
	}

	integration := models.JiraBoardIntegration{BoardID: uint(boardID)}
	err = database.GetDB().Where("board_id = ?", boardID).Assign(models.JiraBoardIntegration{
		BaseURL:       req.BaseURL,
		Email:         req.Email,
		APIToken:      req.APIToken,
		ProjectKey:    req.ProjectKey,
		ConnectedBy:   currentUserID(c),
		ConnectedAt:   time.Now(),
		LastSyncError: "",
	}).FirstOrCreate(&integration).Error
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error connecting Jira integration"})
	}

	go services.RunJiraSync(services.NewRealJiraClient())

	return c.Status(201).JSON(jiraStatusDTO(integration))
}

// Status - GET /api/v1/projects/:id/boards/:boardId/jira-integration
func (h *JiraIntegrationHandler) Status(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var integration models.JiraBoardIntegration
	if err := database.GetDB().Where("board_id = ?", boardID).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(models.JiraIntegrationStatusDTO{Connected: false})
		}
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading Jira integration"})
	}

	return c.JSON(jiraStatusDTO(integration))
}

// Disconnect - DELETE /api/v1/projects/:id/boards/:boardId/jira-integration
// Removes the integration row only; already-mirrored tasks/columns are left
// in place (frozen, no longer synced) rather than deleted.
func (h *JiraIntegrationHandler) Disconnect(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	if err := database.GetDB().Where("board_id = ?", boardID).Delete(&models.JiraBoardIntegration{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error disconnecting Jira integration"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Jira integration disconnected"})
}
