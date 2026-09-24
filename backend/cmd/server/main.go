package main

import (
	"dev-bridge-manager/internal/database"
	"log"
	"os"

	"dev-bridge-manager/internal/middleware"
	"dev-bridge-manager/internal/routes"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL != "" {
		if err := database.RunMigrations(databaseURL); err != nil {
			log.Printf("❌ Migration failed: %v", err)
			// Don't exit - let app start even if migrations fail
		}
	} else {
		log.Println("⚠️  No DATABASE_URL provided, skipping migrations")
	}

	// Connect to database with GORM
	database.Connect()

	// Create Fiber app with config
	app := fiber.New(fiber.Config{
		AppName:      "Dev Bridge Manager v1.0",
		ErrorHandler: errorHandler,
		BodyLimit:    55 * 1024 * 1024, // 55MB — up to 5 attachments x 10MB each, plus multipart overhead
		// fasthttp's default is 4096 bytes; browsers on this dev machine send
		// the "localhost" cookie jar (host-only, not port-scoped) shared across
		// every other project running on localhost, which can push combined
		// request headers past that default and trigger a 431.
		ReadBufferSize: 16 * 1024,
	})

	// Global middleware
	app.Use(logger.New())
	app.Use(middleware.CORS())
	app.Use(recover.New())

	// Setup all routes
	routes.SetupRoutes(app)

	// Monthly auto-invoice notices for hourly projects with it enabled (see
	// services.RunAutoInvoiceNotices)
	go services.StartAutoInvoiceScheduler()

	// Gmail inbox/sent mirror sync, every 3 hours (see services.RunGmailSync)
	go services.StartGmailSyncScheduler()

	// Job search scraping/scoring, once at startup then every 24h (see
	// services.RunScrape)
	go services.StartJobScrapingScheduler()

	// Start server
	port := getPort()
	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("🌍 Environment: %s", getEnv("GO_ENV", "development"))
	log.Printf("📍 Health: http://localhost:%s/health", port)
	log.Printf("📍 API: http://localhost:%s/api/v1", port)

	log.Fatal(app.Listen(":" + port))
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"error":   true,
		"message": err.Error(),
	})
}
