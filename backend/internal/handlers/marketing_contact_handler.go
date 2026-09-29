// backend/internal/handlers/marketing_contact_handler.go
package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
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

// maxImportFileSize is the hard cap on an uploaded CSV's size - the
// import runs synchronously within one HTTP request, so an oversized
// upload is rejected outright rather than accepted and parsed.
const maxImportFileSize = 5 * 1024 * 1024 // 5 MB

// ImportMarketingContacts - POST /api/v1/marketing-contacts/import
// multipart/form-data fields: "files" (the CSV, reusing this
// codebase's existing multi-file-upload wire format), "overwrite"
// ("true"/"false", default "false").
func (h *MarketingContactHandler) ImportMarketingContacts(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("files")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "CSV file is required"})
	}
	if fileHeader.Size > maxImportFileSize {
		return c.Status(413).JSON(fiber.Map{"success": false, "message": "File exceeds the 5 MB limit"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Could not read uploaded file"})
	}
	defer file.Close()

	parsed, err := services.ParseContactsCSV(file)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	overwrite := c.FormValue("overwrite") == "true"
	userID := currentUserID(c)
	result := models.ImportResult{Errors: append([]string{}, parsed.Errors...)}

	for _, row := range parsed.Rows {
		var existing models.MarketingContact
		err := database.GetDB().Where("email = ?", row.Email).First(&existing).Error
		if err != nil {
			contact := models.MarketingContact{
				Email:      row.Email,
				FirstName:  row.FirstName,
				LastName:   row.LastName,
				Subscribed: row.Subscribed,
				Tags:       row.Tags,
				Source:     row.Source,
				Notes:      row.Notes,
				CreatedBy:  userID,
			}
			if err := database.GetDB().Create(&contact).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", row.RowNumber, err))
				continue
			}
			result.Created++
			continue
		}

		if !overwrite {
			result.Skipped++
			continue
		}

		existing.FirstName = row.FirstName
		existing.LastName = row.LastName
		existing.Subscribed = row.Subscribed
		existing.Tags = row.Tags
		existing.Source = row.Source
		existing.Notes = row.Notes
		if err := database.GetDB().Save(&existing).Error; err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", row.RowNumber, err))
			continue
		}
		result.Updated++
	}

	return c.JSON(result)
}

// ExportMarketingContacts - GET /api/v1/marketing-contacts/export?subscribed=&tag=
func (h *MarketingContactHandler) ExportMarketingContacts(c *fiber.Ctx) error {
	query := database.GetDB().Model(&models.MarketingContact{})
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

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"email", "first_name", "last_name", "subscribed", "tags", "source", "notes"})
	for _, contact := range contacts {
		_ = writer.Write([]string{
			contact.Email,
			contact.FirstName,
			contact.LastName,
			strconv.FormatBool(contact.Subscribed),
			contact.Tags,
			contact.Source,
			contact.Notes,
		})
	}
	writer.Flush()

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="marketing-contacts.csv"`)
	return c.Send(buf.Bytes())
}
