package handlers

import (
	"log"
	"os"
	"strings"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type GmailAuthHandler struct{}

func NewGmailAuthHandler() *GmailAuthHandler { return &GmailAuthHandler{} }

// frontendBaseURL reuses the first configured CORS origin rather than
// adding a new env var, since ALLOWED_ORIGINS already names this app's
// frontend origin(s) (see middleware.CORS).
func frontendBaseURL() string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		return "http://localhost:3010"
	}
	return strings.TrimSpace(strings.Split(origins, ",")[0])
}

// AuthURL - GET /api/v1/gmail/auth-url - Google consent URL kérése
func (h *GmailAuthHandler) AuthURL(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	return c.JSON(fiber.Map{"success": true, "url": services.GmailAuthURL(userID)})
}

// Callback - GET /api/v1/gmail/callback - Google redirect ide érkezik;
// nincs JWT middleware ezen az útvonalon (lásd gmail_routes.go), a
// one-time state paraméter védi CSRF ellen.
func (h *GmailAuthHandler) Callback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		return c.Redirect(frontendBaseURL() + "/dashboard/emails?error=oauth_failed")
	}

	if err := services.ExchangeAndSaveGmailAccount(c.Context(), code, state); err != nil {
		log.Printf("gmail oauth callback failed: %v", err)
		return c.Redirect(frontendBaseURL() + "/dashboard/emails?error=oauth_failed")
	}
	return c.Redirect(frontendBaseURL() + "/dashboard/emails?connected=1")
}

// Status - GET /api/v1/gmail/status
func (h *GmailAuthHandler) Status(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var account models.GmailAccount
	err := database.GetDB().Where("user_id = ?", userID).First(&account).Error
	if err != nil {
		return c.JSON(models.GmailStatusResponse{Success: true, Connected: false})
	}

	return c.JSON(models.GmailStatusResponse{
		Success:      true,
		Connected:    true,
		EmailAddress: account.EmailAddress,
		LastSyncedAt: account.LastSyncedAt,
		NeedsReauth:  account.NeedsReauth,
	})
}

// Disconnect - POST /api/v1/gmail/disconnect
func (h *GmailAuthHandler) Disconnect(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	database.GetDB().Where("user_id = ?", userID).Delete(&models.GmailAccount{})
	return c.JSON(fiber.Map{"success": true})
}
