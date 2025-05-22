package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

type UsersHandler struct{}

func NewUsersHandler() *UsersHandler {
	return &UsersHandler{}
}

func (h *UsersHandler) GetUsers(c *fiber.Ctx) error {
	var users []models.User

	result := database.GetDB().Find(&users)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": result.Error.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    users,
		"count":   len(users),
	})
}

func (h *UsersHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User

	result := database.GetDB().First(&user, id)
	if result.Error != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    user,
	})
}
