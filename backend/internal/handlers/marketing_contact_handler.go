// backend/internal/handlers/marketing_contact_handler.go
package handlers

import (
	"strconv"
	"strings"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type MarketingContactHandler struct{}

func NewMarketingContactHandler() *MarketingContactHandler {
	return &MarketingContactHandler{}
}

// ListMarketingContacts - GET /api/v1/marketing-contacts?search=&subscribed=&tag=
func (h *MarketingContactHandler) ListMarketingContacts(c *fiber.Ctx) error {
	query := database.GetDB().Model(&models.MarketingContact{})

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		pattern := ilikePattern(search)
		query = query.Where("email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?", pattern, pattern, pattern)
	}
	if subscribed := c.Query("subscribed"); subscribed == "true" || subscribed == "false" {
		query = query.Where("subscribed = ?", subscribed == "true")
	}
	if tag := strings.TrimSpace(c.Query("tag")); tag != "" {
		query = query.Where("tags ILIKE ?", ilikePattern(tag))
	}

	var contacts []models.MarketingContact
	if err := query.Order("created_at DESC").Find(&contacts).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading marketing contacts"})
	}

	return c.JSON(contacts)
}

// CreateMarketingContact - POST /api/v1/marketing-contacts
func (h *MarketingContactHandler) CreateMarketingContact(c *fiber.Ctx) error {
	var req models.MarketingContactCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	email := strings.TrimSpace(req.Email)
	if email == "" || !services.IsValidEmail(email) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Valid email is required"})
	}

	var existing models.MarketingContact
	if err := database.GetDB().Where("email = ?", email).First(&existing).Error; err == nil {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": "A contact with this email already exists"})
	}

	subscribed := true
	if req.Subscribed != nil {
		subscribed = *req.Subscribed
	}

	contact := models.MarketingContact{
		Email:      email,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Subscribed: subscribed,
		Tags:       req.Tags,
		Source:     req.Source,
		Notes:      req.Notes,
		CreatedBy:  currentUserID(c),
	}
	if err := database.GetDB().Create(&contact).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating contact"})
	}

	return c.Status(201).JSON(contact)
}

// UpdateMarketingContact - PUT /api/v1/marketing-contacts/:id
func (h *MarketingContactHandler) UpdateMarketingContact(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid contact ID"})
	}

	var contact models.MarketingContact
	if err := database.GetDB().First(&contact, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Contact not found"})
	}

	var req models.MarketingContactUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	email := strings.TrimSpace(req.Email)
	if email == "" || !services.IsValidEmail(email) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Valid email is required"})
	}

	if email != contact.Email {
		var existing models.MarketingContact
		if err := database.GetDB().Where("email = ? AND id != ?", email, id).First(&existing).Error; err == nil {
			return c.Status(409).JSON(fiber.Map{"success": false, "message": "A contact with this email already exists"})
		}
	}

	subscribed := contact.Subscribed
	if req.Subscribed != nil {
		subscribed = *req.Subscribed
	}

	contact.Email = email
	contact.FirstName = req.FirstName
	contact.LastName = req.LastName
	contact.Subscribed = subscribed
	contact.Tags = req.Tags
	contact.Source = req.Source
	contact.Notes = req.Notes

	if err := database.GetDB().Save(&contact).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating contact"})
	}

	return c.JSON(contact)
}

// DeleteMarketingContact - DELETE /api/v1/marketing-contacts/:id
func (h *MarketingContactHandler) DeleteMarketingContact(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid contact ID"})
	}

	result := database.GetDB().Delete(&models.MarketingContact{}, id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting contact"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Contact not found"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Contact deleted successfully"})
}
