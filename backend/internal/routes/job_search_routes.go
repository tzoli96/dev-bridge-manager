// backend/internal/routes/job_search_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupJobSearchRoutes(api fiber.Router) {
	h := handlers.NewJobSearchHandler()

	admin := api.Group("/admin/job-search")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only, same reasoning as profile_routes.go: this is a
	// personal job-search tool for the operator, not a client-facing
	// feature - passed per-route, not via admin.Use(), so it never leaks
	// onto other /admin/* routes registered by other route files.
	admin.Get("/profile", middleware.RequireRole("super_admin"), h.GetJobSearchProfile)
	admin.Put("/profile", middleware.RequireRole("super_admin"), h.UpdateJobSearchProfile)
	admin.Post("/scan-now", middleware.RequireRole("super_admin"), h.ScanNow)
	admin.Post("/listings/manual", middleware.RequireRole("super_admin"), h.AddManualListing)
	admin.Get("/matches", middleware.RequireRole("super_admin"), h.ListJobMatches)
	admin.Patch("/matches/:id/status", middleware.RequireRole("super_admin"), h.UpdateJobMatchStatus)
	admin.Post("/matches/:id/draft-application", middleware.RequireRole("super_admin"), h.DraftApplication)
	admin.Post("/matches/:id/mark-applied", middleware.RequireRole("super_admin"), h.MarkApplied)
}
