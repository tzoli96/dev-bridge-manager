package handlers

import "github.com/gofiber/fiber/v2"

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Check(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "ok",
		"message":   "Dev Bridge Manager is running!",
		"version":   "1.0.0",
		"timestamp": c.Context().Time(),
	})
}

func (h *HealthHandler) Ping(c *fiber.Ctx) error {
	return c.SendString("pong")
}
