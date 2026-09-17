// backend/internal/routes/invoice_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInvoiceRoutes(api fiber.Router) {
	invoiceHandler := handlers.NewInvoiceHandler()
	noticeHandler := handlers.NewInvoiceNoticeHandler()

	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// POST /api/v1/projects/:id/invoices - Számla kiállítása egy projekthez
	projects.Post("/:id/invoices", invoiceHandler.CreateInvoice)

	// GET /api/v1/projects/:id/invoices - Projekt számla-előzményeinek listázása
	projects.Get("/:id/invoices", invoiceHandler.GetProjectInvoices)

	// POST /api/v1/projects/:id/invoice-notice - Számla-értesítő e-mail küldése az ügyfélnek
	projects.Post("/:id/invoice-notice", noticeHandler.SendInvoiceNotice)
	// GET /api/v1/projects/:id/invoice-notices - Elküldött értesítők listázása
	projects.Get("/:id/invoice-notices", noticeHandler.ListInvoiceNotices)

	// GET /api/v1/projects/:id/invoices/analytics - Projekt bevétel elemzése (havi/éves/összesen)
	projects.Get("/:id/invoices/analytics", invoiceHandler.GetProjectRevenueAnalytics)

	// GET /api/v1/projects/:id/invoices/:invoiceId/breakdown - Mely órák/feladatok lettek kiszámlázva
	projects.Get("/:id/invoices/:invoiceId/breakdown", invoiceHandler.GetInvoiceBreakdown)

	// GET /api/v1/projects/:id/invoices/:invoiceId/pdf - Számla PDF megtekintése
	projects.Get("/:id/invoices/:invoiceId/pdf", invoiceHandler.DownloadInvoicePDF)

	// POST /api/v1/projects/:id/invoices/refresh-payment-status - Fizetettség frissítése Billingóból
	projects.Post("/:id/invoices/refresh-payment-status", invoiceHandler.RefreshPaymentStatuses)

	clients := api.Group("/clients")
	clients.Use(middleware.JWTMiddleware())

	// GET /api/v1/clients/:id/invoices/analytics - Ügyfél bevétel elemzése (havi/éves/összesen, minden projektjén)
	clients.Get("/:id/invoices/analytics", invoiceHandler.GetClientRevenueAnalytics)
}
