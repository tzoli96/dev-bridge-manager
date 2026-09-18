# Invoice Approval Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Insert an internal approval step between the client-facing pre-invoice notice e-mail and actual Billingo invoice creation, so a team member must approve a notice in-app before the invoice is generated — and, only for invoices created this way, automatically e-mail the client the invoice PDF afterward.

**Architecture:** Extract the existing `CreateInvoice` handler's pricing/Billingo/persistence logic into a shared `createInvoiceForProject` function so both the manual "Számla kiállítása" button and the new approval endpoint call the exact same code. Extract the existing notice-sending logic into a `services.SendInvoiceNoticeEmail` function so both the manual "Értesítő küldése" button and the monthly scheduler (which has no logged-in user) can send notices. Add `Status`/`InvoiceID`/`ApprovedBy`/`ApprovedAt` to `InvoiceNotice` to track the approval state machine (`pending` → `approved`). The scheduler (`auto_invoice_enabled` toggle) now sends notices automatically at month start instead of creating invoices automatically; invoice creation always requires either the manual button or an explicit approval click.

**Tech Stack:** Go + Fiber + GORM + Postgres (backend, `dev-bridge-manager/backend`), Next.js + React + TypeScript (frontend, `dev-bridge-manager/frontend`), Billingo API (`services.BillingoService`), Gmail API (`services.RealGmailAPI`), Docker Compose (`devbridge_backend`, `devbridge_frontend`, `devbridge_postgres` containers).

**Spec:** `docs/superpowers/specs/2026-09-18-invoice-approval-flow-design.md`

## Global Constraints

- All backend Go commands run inside the running container: `docker exec devbridge_backend go build ./...`, `docker exec devbridge_backend go vet ./...`, `docker exec devbridge_backend go test ./...`. Never run `go` on the host.
- All frontend commands run inside the running container: `docker exec devbridge_frontend npx tsc --noEmit`. Never run `npm`/`npx` on the host.
- Migrations auto-apply on backend startup (`database.RunMigrations`) — after adding a migration file, restart the backend container (`docker restart devbridge_backend`) to apply it, then verify with `docker exec devbridge_postgres psql -U postgres -d devbridge -c "\d invoice_notices"` (adjust user/db name to match `.docker/docker-compose.yml` if different).
- The dev database has **real, live** Billingo and Gmail credentials configured. Do not trigger a real Billingo invoice creation or a real Gmail send during automated verification — verification is limited to `go build`/`go vet`/`go test`/`tsc --noEmit` plus genuine unit tests of pure logic. Any end-to-end check that would create a real invoice or send a real e-mail is out of scope for this plan's automated steps; call it out to the user as a manual follow-up instead.
- Next.js dates use `YYYY-MM-DD` (`"2006-01-02"` in Go `time.Parse`/`Format`); keep this format for all new period fields.
- New DB migration is `000025_add_invoice_notice_approval` (next number after the existing `000024_add_gmail_integration`).
- Existing manual invoice creation (`POST /projects/:id/invoices` → `CreateInvoice`) must keep working unchanged and must never trigger the post-approval PDF e-mail.
- Keep all new/changed user-facing strings in Hungarian, matching the existing copy style in the touched files.
- Do not commit unless explicitly asked; each task step below still includes a `git commit` step per this skill's convention — run it, since committing after each task is this repo's established workflow, but do not push.

---

### Task 1: Migration — invoice_notices approval columns

**Files:**
- Create: `backend/migrations/000025_add_invoice_notice_approval.up.sql`
- Create: `backend/migrations/000025_add_invoice_notice_approval.down.sql`

**Interfaces:**
- Produces: `invoice_notices` table gains `status VARCHAR(20) NOT NULL DEFAULT 'pending'`, `invoice_id INTEGER REFERENCES invoices(id) ON DELETE SET NULL`, `approved_by INTEGER REFERENCES users(id)`, `approved_at TIMESTAMP`, plus index `idx_invoice_notices_status`. Task 2 maps these onto new `InvoiceNotice` struct fields.

- [ ] **Step 1: Write the up migration**

```sql
-- backend/migrations/000025_add_invoice_notice_approval.up.sql
ALTER TABLE invoice_notices
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN invoice_id INTEGER REFERENCES invoices(id) ON DELETE SET NULL,
    ADD COLUMN approved_by INTEGER REFERENCES users(id),
    ADD COLUMN approved_at TIMESTAMP;

CREATE INDEX idx_invoice_notices_status ON invoice_notices (status);
```

- [ ] **Step 2: Write the down migration**

```sql
-- backend/migrations/000025_add_invoice_notice_approval.down.sql
DROP INDEX IF EXISTS idx_invoice_notices_status;

ALTER TABLE invoice_notices
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS invoice_id,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at;
```

- [ ] **Step 3: Apply and verify**

Run: `docker restart devbridge_backend`
Then: `docker exec devbridge_postgres psql -U postgres -d devbridge -c "\d invoice_notices"` (use the actual DB user/name from `.docker/docker-compose.yml` if this fails)
Expected: output lists `status`, `invoice_id`, `approved_by`, `approved_at` columns and `idx_invoice_notices_status` index.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/000025_add_invoice_notice_approval.up.sql backend/migrations/000025_add_invoice_notice_approval.down.sql
git commit -m "$(cat <<'EOF'
feat(invoices): add approval columns to invoice_notices

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: InvoiceNotice model — approval fields

**Files:**
- Modify: `backend/internal/models/invoice_notice.go`

**Interfaces:**
- Consumes: migration columns from Task 1 (`status`, `invoice_id`, `approved_by`, `approved_at`).
- Produces: `models.InvoiceNotice.Status string`, `.InvoiceID *uint`, `.ApprovedBy *uint`, `.ApprovedAt *time.Time`; new `models.InvoiceNoticeWithNames` (embeds `InvoiceNotice` + `ProjectName string` + `ClientName string`) for Task 9's list endpoint; new `models.InvoiceNoticeApproveResponse{Success bool, Message string, Invoice *InvoiceResponse, Notice *InvoiceNotice, EmailSent bool}` for Task 8's approve endpoint.

- [ ] **Step 1: Replace the file contents**

```go
// backend/internal/models/invoice_notice.go
package models

import "time"

type InvoiceNotice struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	ProjectID      uint       `json:"project_id" gorm:"not null"`
	ClientID       uint       `json:"client_id" gorm:"not null"`
	PeriodStart    *time.Time `json:"period_start"`
	PeriodEnd      *time.Time `json:"period_end"`
	GmailMessageID string     `json:"gmail_message_id" gorm:"size:100;not null"`
	SentBy         uint       `json:"sent_by" gorm:"not null"`
	SentAt         time.Time  `json:"sent_at"`
	Status         string     `json:"status" gorm:"size:20;not null;default:'pending'"`
	InvoiceID      *uint      `json:"invoice_id"`
	ApprovedBy     *uint      `json:"approved_by"`
	ApprovedAt     *time.Time `json:"approved_at"`
}

func (InvoiceNotice) TableName() string { return "invoice_notices" }

// InvoiceNoticeWithNames adds the project/client display names that
// ListAllInvoiceNotices joins in, for the Billing page's pending-notices list.
type InvoiceNoticeWithNames struct {
	InvoiceNotice
	ProjectName string `json:"project_name"`
	ClientName  string `json:"client_name"`
}

type InvoiceNoticeSendRequest struct {
	ClientID    uint   `json:"client_id"`
	PeriodStart string `json:"period_start,omitempty"`
	PeriodEnd   string `json:"period_end,omitempty"`
}

type InvoiceNoticeResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	Notice  *InvoiceNotice `json:"notice,omitempty"`
}

type InvoiceNoticeListResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Notices []InvoiceNotice `json:"notices,omitempty"`
}

// InvoiceNoticeApproveResponse is returned by
// InvoiceNoticeHandler.ApproveInvoiceNotice. EmailSent is false whenever the
// invoice was created successfully but the invoice-ready e-mail could not be
// sent (e.g. the approver has no connected Gmail account) — Message then
// carries a human-readable warning while Success stays true, since the
// invoice itself was created.
type InvoiceNoticeApproveResponse struct {
	Success   bool             `json:"success"`
	Message   string           `json:"message,omitempty"`
	Invoice   *InvoiceResponse `json:"invoice,omitempty"`
	Notice    *InvoiceNotice   `json:"notice,omitempty"`
	EmailSent bool             `json:"email_sent"`
}
```

- [ ] **Step 2: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds successfully (no other file references the old struct shape yet in a way that breaks).

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/invoice_notice.go
git commit -m "$(cat <<'EOF'
feat(invoices): add approval fields to InvoiceNotice model

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Extract `createInvoiceForProject` from `CreateInvoice`

**Files:**
- Modify: `backend/internal/handlers/invoice_handler.go`

**Interfaces:**
- Consumes: `services.BillingoService` (`EnsurePartner`, `CreateInvoice`), `models.Project`, `models.Client`, `models.InvoiceCreateRequest`, `services.CalculateFixedAmount`, `services.CalculateHourlyAmount`, `services.SumLoggedHours`, `services.FriendlyBillingoError`, `services.InvoiceLineItem`, `isDuplicateKeyError`.
- Produces: `func createInvoiceForProject(billingoService *services.BillingoService, project models.Project, client models.Client, req models.InvoiceCreateRequest, createdBy uint) (invoice *models.Invoice, items []models.InvoiceItem, httpStatus int, message string)` — `httpStatus == 0` means success; a non-zero `httpStatus`/`message` pair means failure, ready to write straight into a fiber JSON response. Also produces package-level (no longer method) functions `recordFailedInvoice(projectID, clientID, createdBy uint, pricingType string, periodStart, periodEnd *time.Time, amount float64, errMsg string)`, `markReservationCreated(invoice *models.Invoice, billingoInvoiceID, billingoInvoiceNumber string) error`, `markReservationFailed(invoice *models.Invoice, errMsg string)` — Task 8's `ApproveInvoiceNotice` calls `createInvoiceForProject` directly (it has no `*InvoiceHandler`, only a `*services.BillingoService`).

- [ ] **Step 1: Convert the three helper methods to package-level functions**

In `backend/internal/handlers/invoice_handler.go`, change:

```go
func (h *InvoiceHandler) recordFailedInvoice(projectID, clientID, createdBy uint, pricingType string, periodStart, periodEnd *time.Time, amount float64, errMsg string) {
```

to:

```go
func recordFailedInvoice(projectID, clientID, createdBy uint, pricingType string, periodStart, periodEnd *time.Time, amount float64, errMsg string) {
```

(body unchanged). Do the same for `markReservationCreated` and `markReservationFailed`:

```go
func markReservationCreated(invoice *models.Invoice, billingoInvoiceID, billingoInvoiceNumber string) error {
```

```go
func markReservationFailed(invoice *models.Invoice, errMsg string) {
```

(bodies unchanged — none of the three use `h`).

- [ ] **Step 2: Insert `createInvoiceForProject` right after `markReservationFailed` and before `CreateInvoice`**

```go
// createInvoiceForProject runs the shared pricing/Billingo/persistence logic
// for creating an invoice. Used by both the manual CreateInvoice handler and
// InvoiceNoticeHandler.ApproveInvoiceNotice, so the two paths can never
// diverge in how an invoice is priced or persisted. Assumes the caller has
// already resolved project/client and validated req.ClientID != 0.
// httpStatus == 0 means success (invoice/items are set); otherwise
// httpStatus/message are ready to write straight into a fiber response.
func createInvoiceForProject(billingoService *services.BillingoService, project models.Project, client models.Client, req models.InvoiceCreateRequest, createdBy uint) (invoice *models.Invoice, items []models.InvoiceItem, httpStatus int, message string) {
	projectID := project.ID

	var dueDate *time.Time
	if req.DueDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			return nil, nil, 400, "Invalid due_date (expected YYYY-MM-DD)"
		}
		dueDate = &parsed
	}

	var amount float64
	var periodStart, periodEnd *time.Time
	var description string
	baseQuantity := 1.0
	var baseUnit string
	var baseUnitPrice float64
	var reservation *models.Invoice

	switch project.PricingType {
	case "fixed":
		if project.FixedPrice == nil {
			return nil, nil, 400, "Project has no fixed price configured"
		}

		fixedPrice := *project.FixedPrice
		if req.BaseUnitPrice > 0 {
			fixedPrice = req.BaseUnitPrice
		}
		amount = services.CalculateFixedAmount(fixedPrice)
		baseUnit = client.BillingoUnit
		baseUnitPrice = amount
		description = fmt.Sprintf("%s - fixed price", project.Name)

		reservation = &models.Invoice{
			ProjectID:   projectID,
			ClientID:    req.ClientID,
			PricingType: "fixed",
			Amount:      amount,
			Status:      "pending",
			CreatedBy:   createdBy,
		}
		if err := database.GetDB().Create(reservation).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, nil, 409, "Ez a projekt már ki lett számlázva"
			}
			return nil, nil, 500, "Failed to reserve invoice slot"
		}

	case "hourly":
		if project.HourlyRate == nil {
			return nil, nil, 400, "Project has no hourly rate configured"
		}
		if req.PeriodStart == "" || req.PeriodEnd == "" {
			return nil, nil, 400, "period_start and period_end are required for hourly projects"
		}

		start, err := time.Parse("2006-01-02", req.PeriodStart)
		if err != nil {
			return nil, nil, 400, "Invalid period_start (expected YYYY-MM-DD)"
		}
		end, err := time.Parse("2006-01-02", req.PeriodEnd)
		if err != nil {
			return nil, nil, 400, "Invalid period_end (expected YYYY-MM-DD)"
		}
		if start.After(end) {
			return nil, nil, 400, "period_start must not be after period_end"
		}

		totalHours, err := services.SumLoggedHours(projectID, start, end)
		if err != nil {
			return nil, nil, 500, "Error summing logged hours"
		}

		hourlyRate := *project.HourlyRate
		if req.BaseUnitPrice > 0 {
			hourlyRate = req.BaseUnitPrice
		}
		amount = services.CalculateHourlyAmount(totalHours, hourlyRate)
		baseQuantity = totalHours
		baseUnit = "óra"
		baseUnitPrice = hourlyRate
		periodStart, periodEnd = &start, &end
		description = fmt.Sprintf("%s - %s to %s", project.Name, req.PeriodStart, req.PeriodEnd)

	default:
		return nil, nil, 400, "Unsupported pricing type"
	}

	if req.ItemName != "" {
		description = req.ItemName
	}

	type invoiceLine struct {
		Name      string
		Quantity  float64
		Unit      string
		UnitPrice float64
		IsBase    bool
	}
	lines := []invoiceLine{
		{Name: description, Quantity: baseQuantity, Unit: baseUnit, UnitPrice: baseUnitPrice, IsBase: true},
	}
	for _, extra := range req.ExtraItems {
		name := strings.TrimSpace(extra.Name)
		if name == "" && extra.Quantity == 0 && extra.UnitPrice == 0 {
			continue
		}
		if name == "" {
			return nil, nil, 400, "Extra item name is required"
		}
		if extra.Quantity <= 0 {
			return nil, nil, 400, "Extra item quantity must be greater than 0"
		}
		lines = append(lines, invoiceLine{Name: name, Quantity: extra.Quantity, Unit: extra.Unit, UnitPrice: extra.UnitPrice})
		amount += extra.Quantity * extra.UnitPrice
	}

	if reservation != nil {
		reservation.ItemName = description
		reservation.DueDate = dueDate
		reservation.Amount = amount
		if err := database.GetDB().Model(reservation).Updates(map[string]interface{}{
			"item_name": reservation.ItemName,
			"due_date":  reservation.DueDate,
			"amount":    reservation.Amount,
		}).Error; err != nil {
			markReservationFailed(reservation, "failed to save item name/due date")
			return nil, nil, 500, "Failed to reserve invoice slot"
		}
	}

	billingoItems := make([]services.InvoiceLineItem, 0, len(lines))
	for _, l := range lines {
		billingoItems = append(billingoItems, services.InvoiceLineItem{
			Name: l.Name, Quantity: l.Quantity, Unit: l.Unit, UnitPrice: l.UnitPrice, UnitPriceType: client.BillingoUnitPriceType,
		})
	}

	partnerID, err := billingoService.EnsurePartner(&client)
	if err != nil {
		friendly := services.FriendlyBillingoError(err)
		if reservation != nil {
			markReservationFailed(reservation, friendly)
		} else {
			recordFailedInvoice(projectID, req.ClientID, createdBy, project.PricingType, periodStart, periodEnd, amount, friendly)
		}
		return nil, nil, 502, friendly
	}

	billingoInvoiceID, billingoInvoiceNumber, err := billingoService.CreateInvoice(partnerID, billingoItems, req.DueDate)
	if err != nil {
		friendly := services.FriendlyBillingoError(err)
		if reservation != nil {
			markReservationFailed(reservation, friendly)
		} else {
			recordFailedInvoice(projectID, req.ClientID, createdBy, project.PricingType, periodStart, periodEnd, amount, friendly)
		}
		return nil, nil, 502, friendly
	}

	var inv models.Invoice
	if reservation != nil {
		if err := markReservationCreated(reservation, billingoInvoiceID, billingoInvoiceNumber); err != nil {
			log.Printf("⚠️ Failed to finalize invoice reservation %d as created (project %d): %v", reservation.ID, reservation.ProjectID, err)
			return nil, nil, 500, "Invoice created in Billingo but failed to save locally"
		}
		inv = *reservation
	} else {
		inv = models.Invoice{
			ProjectID:             projectID,
			ClientID:              req.ClientID,
			BillingoInvoiceID:     billingoInvoiceID,
			BillingoInvoiceNumber: billingoInvoiceNumber,
			PricingType:           project.PricingType,
			PeriodStart:           periodStart,
			PeriodEnd:             periodEnd,
			ItemName:              description,
			DueDate:               dueDate,
			Amount:                amount,
			Status:                "created",
			CreatedBy:             createdBy,
		}
		if err := database.GetDB().Create(&inv).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, nil, 409, "Ez a projekt már ki lett számlázva"
			}
			return nil, nil, 500, "Invoice created in Billingo but failed to save locally"
		}
	}

	itemRows := make([]models.InvoiceItem, 0, len(lines))
	for _, l := range lines {
		itemRows = append(itemRows, models.InvoiceItem{
			InvoiceID:     inv.ID,
			Name:          l.Name,
			Quantity:      l.Quantity,
			Unit:          l.Unit,
			UnitPrice:     l.UnitPrice,
			UnitPriceType: client.BillingoUnitPriceType,
			LineTotal:     l.Quantity * l.UnitPrice,
			IsBase:        l.IsBase,
		})
	}
	if err := database.GetDB().Create(&itemRows).Error; err != nil {
		log.Printf("⚠️ Failed to save invoice_items for invoice %d: %v", inv.ID, err)
		itemRows = nil
	}

	return &inv, itemRows, 0, ""
}
```

- [ ] **Step 3: Slim `CreateInvoice` down to call the extracted function**

Replace the body of `CreateInvoice` (everything from `var dueDate *time.Time` through the final `return c.Status(201)...`) so the whole function reads:

```go
// CreateInvoice - POST /api/v1/projects/:id/invoices
func (h *InvoiceHandler) CreateInvoice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid project ID"})
	}

	var project models.Project
	if err := database.GetDB().First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceListResponse{Success: false, Message: "Project not found"})
	}

	if project.PricingType == "" {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no pricing type configured"})
	}

	var req models.InvoiceCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid request body"})
	}

	if req.ClientID == 0 {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Client ID is required"})
	}

	var projectClient models.ProjectClient
	if err := database.GetDB().Where("project_id = ? AND client_id = ?", projectID, req.ClientID).First(&projectClient).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Client is not attached to this project"})
	}

	var client models.Client
	if err := database.GetDB().First(&client, req.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceListResponse{Success: false, Message: "Client not found"})
	}

	invoice, items, httpStatus, message := createInvoiceForProject(h.billingoService, project, client, req, currentUserID)
	if httpStatus != 0 {
		return c.Status(httpStatus).JSON(models.InvoiceListResponse{Success: false, Message: message})
	}

	response := toInvoiceResponse(*invoice, client.Name, "")
	response.Items = items

	return c.Status(201).JSON(models.InvoiceListResponse{
		Success: true,
		Message: "Invoice created successfully",
		Invoice: ptrInvoiceResponse(response),
	})
}
```

- [ ] **Step 4: Build and vet**

Run: `docker exec devbridge_backend go build ./...` then `docker exec devbridge_backend go vet ./...`
Expected: both succeed with no errors (in particular, no "declared and not used" for `dueDate`/`amount`/etc. — they now live only inside `createInvoiceForProject`).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/invoice_handler.go
git commit -m "$(cat <<'EOF'
refactor(invoices): extract createInvoiceForProject from CreateInvoice

Shares the pricing/Billingo/persistence logic with the upcoming
notice-approval endpoint instead of duplicating it.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `services.BuildInvoiceNoticeText` + `services.SendInvoiceNoticeEmail`

**Files:**
- Create: `backend/internal/services/invoice_notice.go`
- Create: `backend/internal/services/invoice_notice_test.go`

**Interfaces:**
- Consumes: `services.BuildRawMessage(fromAddress, to, subject, bodyText, bodyHTML, inReplyToHeader, referencesHeader string, attachments []MessageAttachment) []byte`, `services.NewRealGmailAPI().SendMessage(ctx, *models.GmailAccount, raw []byte) (string, error)`, `database.GetDB()`.
- Produces: `func BuildInvoiceNoticeText(clientName, projectName string, periodStart, periodEnd *time.Time) (subject, body string)`; `func SendInvoiceNoticeEmail(project models.Project, client models.Client, account models.GmailAccount, periodStart, periodEnd *time.Time, sentBy uint) (*models.InvoiceNotice, error)` — Task 5 (slimmed `SendInvoiceNotice` handler) and Task 6 (`autoNotifyProject`) both call `SendInvoiceNoticeEmail`.

- [ ] **Step 1: Write the failing tests**

```go
// backend/internal/services/invoice_notice_test.go
package services

import (
	"testing"
	"time"
)

func TestBuildInvoiceNoticeTextWithoutPeriod(t *testing.T) {
	subject, body := BuildInvoiceNoticeText("Acme Kft.", "Website Redesign", nil, nil)

	wantSubject := "Számla értesítő - Website Redesign"
	if subject != wantSubject {
		t.Fatalf("expected subject %q, got %q", wantSubject, subject)
	}

	wantBody := "Kedves Acme Kft.!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"Website Redesign\" projekt kapcsán.\n\nÜdvözlettel"
	if body != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}

func TestBuildInvoiceNoticeTextWithPeriod(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

	_, body := BuildInvoiceNoticeText("Acme Kft.", "Website Redesign", &start, &end)

	wantBody := "Kedves Acme Kft.!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"Website Redesign\" projekt kapcsán a 2026.08.01 - 2026.08.31 időszakra vonatkozóan.\n\nÜdvözlettel"
	if body != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, body)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestBuildInvoiceNoticeText -v`
Expected: FAIL — `undefined: BuildInvoiceNoticeText`.

- [ ] **Step 3: Implement `invoice_notice.go`**

```go
// backend/internal/services/invoice_notice.go
package services

import (
	"context"
	"fmt"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// BuildInvoiceNoticeText composes the Hungarian subject/body for the
// pre-invoice notice e-mail. Pure function so it can be unit-tested without a
// fake Gmail server.
func BuildInvoiceNoticeText(clientName, projectName string, periodStart, periodEnd *time.Time) (subject, body string) {
	periodText := ""
	if periodStart != nil && periodEnd != nil {
		periodText = fmt.Sprintf(" a %s - %s időszakra vonatkozóan", periodStart.Format("2006.01.02"), periodEnd.Format("2006.01.02"))
	}
	subject = fmt.Sprintf("Számla értesítő - %s", projectName)
	body = fmt.Sprintf(
		"Kedves %s!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"%s\" projekt kapcsán%s.\n\nÜdvözlettel",
		clientName, projectName, periodText,
	)
	return subject, body
}

// SendInvoiceNoticeEmail sends the pre-invoice notice e-mail from the given
// Gmail account to the client, then persists the resulting 'pending'
// InvoiceNotice row. Used both by the manual "Értesítő küldése" button
// (InvoiceNoticeHandler.SendInvoiceNotice) and by the monthly scheduler
// (RunAutoInvoiceNotices), which has no logged-in user to send from — hence
// account/sentBy are passed in explicitly rather than read from request
// context.
func SendInvoiceNoticeEmail(project models.Project, client models.Client, account models.GmailAccount, periodStart, periodEnd *time.Time, sentBy uint) (*models.InvoiceNotice, error) {
	subject, body := BuildInvoiceNoticeText(client.Name, project.Name, periodStart, periodEnd)
	raw := BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "", "", nil)
	gmailMessageID, err := NewRealGmailAPI().SendMessage(context.Background(), &account, raw)
	if err != nil {
		return nil, err
	}

	notice := models.InvoiceNotice{
		ProjectID:      project.ID,
		ClientID:       client.ID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		GmailMessageID: gmailMessageID,
		SentBy:         sentBy,
		SentAt:         time.Now(),
		Status:         "pending",
	}
	if err := database.GetDB().Create(&notice).Error; err != nil {
		return nil, err
	}
	return &notice, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestBuildInvoiceNoticeText -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: succeeds.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/invoice_notice.go backend/internal/services/invoice_notice_test.go
git commit -m "$(cat <<'EOF'
feat(invoices): extract BuildInvoiceNoticeText/SendInvoiceNoticeEmail

Shared between the manual notice button and the monthly scheduler, which
has no logged-in user to send from.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Slim `SendInvoiceNotice` handler to use `services.SendInvoiceNoticeEmail`

**Files:**
- Modify: `backend/internal/handlers/invoice_notice_handler.go`

**Interfaces:**
- Consumes: `services.SendInvoiceNoticeEmail` (Task 4).
- Produces: unchanged `SendInvoiceNotice` HTTP contract (`POST /projects/:id/invoice-notice`, same request/response shapes) — Task 8/9 do not depend on internals of this handler.

- [ ] **Step 1: Replace the body of `SendInvoiceNotice`**

Replace the whole function (everything from the subject/body building through the final `db.Create(&notice)` + response) with:

```go
// SendInvoiceNotice - POST /api/v1/projects/:id/invoice-notice - e-mail
// értesítő küldése az ügyfélnek, a tényleges számla kiállítása előtt.
func (h *InvoiceNoticeHandler) SendInvoiceNotice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid project id"})
	}

	var req models.InvoiceNoticeSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid request body"})
	}
	if req.ClientID == 0 {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "client_id is required"})
	}

	db := database.GetDB()

	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Project not found"})
	}

	var projectClient models.ProjectClient
	if err := db.Where("project_id = ? AND client_id = ?", projectID, req.ClientID).First(&projectClient).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Client is not attached to this project"})
	}

	var client models.Client
	if err := db.First(&client, req.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Client not found"})
	}
	if client.Email == "" {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Client has no email address on file"})
	}

	var account models.GmailAccount
	if err := db.Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	var periodStart, periodEnd *time.Time
	if req.PeriodStart != "" {
		t, err := time.Parse("2006-01-02", req.PeriodStart)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid period_start"})
		}
		periodStart = &t
	}
	if req.PeriodEnd != "" {
		t, err := time.Parse("2006-01-02", req.PeriodEnd)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Invalid period_end"})
		}
		periodEnd = &t
	}

	notice, err := services.SendInvoiceNoticeEmail(project, client, account, periodStart, periodEnd, currentUserID)
	if err != nil {
		return c.Status(502).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Failed to send notice email: " + err.Error()})
	}

	return c.JSON(models.InvoiceNoticeResponse{Success: true, Notice: notice})
}
```

- [ ] **Step 2: Drop the now-unused `fmt` import**

In the `import` block at the top of `backend/internal/handlers/invoice_notice_handler.go`, remove the `"fmt"` line (subject/body building moved into `services.BuildInvoiceNoticeText`).

- [ ] **Step 3: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: succeeds (no unused-import error, no leftover unused locals).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/invoice_notice_handler.go
git commit -m "$(cat <<'EOF'
refactor(invoices): SendInvoiceNotice delegates to SendInvoiceNoticeEmail

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Scheduler sends notices instead of creating invoices

**Files:**
- Modify: `backend/internal/services/auto_invoice.go` (full rewrite of contents)
- Modify: `backend/internal/services/scheduler.go`

**Interfaces:**
- Consumes: `services.SumLoggedHours`, `services.SendInvoiceNoticeEmail` (Task 4), `models.Project`, `models.Client`, `models.GmailAccount`, `models.InvoiceNotice`, `models.Invoice`.
- Produces: `func RunAutoInvoiceNotices()` (replaces `RunAutoInvoicing`) — Task 7's route registrations are unaffected, but `scheduler.go`'s `checkAndRun` now calls this instead.

- [ ] **Step 1: Replace `auto_invoice.go` entirely**

```go
// backend/internal/services/auto_invoice.go
package services

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"log"
	"time"
)

// RunAutoInvoiceNotices sends a pre-invoice notice e-mail (never creates the
// invoice itself) for the previous calendar month, for every hourly project
// with auto-invoicing enabled, to its configured client. Actual invoice
// creation only happens once a team member approves the resulting notice via
// InvoiceNoticeHandler.ApproveInvoiceNotice.
//
// It is safe to call more than once on the same day (see
// StartAutoInvoiceScheduler): each project is skipped if a notice or invoice
// already exists for that exact project/period, so re-running never sends a
// duplicate notice.
func RunAutoInvoiceNotices() {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-24 * time.Hour)

	var projects []models.Project
	if err := database.GetDB().
		Where("auto_invoice_enabled = ? AND pricing_type = ? AND auto_invoice_client_id IS NOT NULL", true, "hourly").
		Find(&projects).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to load eligible projects: %v", err)
		return
	}

	for _, project := range projects {
		autoNotifyProject(project, periodStart, periodEnd)
	}
}

func autoNotifyProject(project models.Project, periodStart, periodEnd time.Time) {
	db := database.GetDB()

	var existingNotice models.InvoiceNotice
	if err := db.Where(
		"project_id = ? AND client_id = ? AND period_start = ? AND period_end = ?",
		project.ID, *project.AutoInvoiceClientID, periodStart, periodEnd,
	).First(&existingNotice).Error; err == nil {
		return // notice already sent for this project/period
	}

	var existingInvoice models.Invoice
	if err := db.Where(
		"project_id = ? AND period_start = ? AND period_end = ? AND status IN ('created','pending')",
		project.ID, periodStart, periodEnd,
	).First(&existingInvoice).Error; err == nil {
		return // already invoiced (e.g. via the manual button) for this period
	}

	if project.HourlyRate == nil {
		log.Printf("⚠️ Auto-invoicing: project %d has no hourly rate configured, skipping", project.ID)
		return
	}

	var client models.Client
	if err := db.First(&client, *project.AutoInvoiceClientID).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: project %d's configured client %d not found, skipping", project.ID, *project.AutoInvoiceClientID)
		return
	}

	totalHours, err := SumLoggedHours(project.ID, periodStart, periodEnd)
	if err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to sum hours for project %d: %v", project.ID, err)
		return
	}
	if totalHours <= 0 {
		log.Printf("ℹ️ Auto-invoicing: project %d has no logged hours for %s, skipping", project.ID, periodStart.Format("2006-01"))
		return
	}

	var account models.GmailAccount
	if err := db.Where("user_id = ?", project.CreatedBy).First(&account).Error; err != nil {
		log.Printf("⚠️ Auto-invoicing: project %d's creator (user %d) has no connected Gmail account, skipping notice", project.ID, project.CreatedBy)
		return
	}

	if _, err := SendInvoiceNoticeEmail(project, client, account, &periodStart, &periodEnd, project.CreatedBy); err != nil {
		log.Printf("⚠️ Auto-invoicing: failed to send notice for project %d: %v", project.ID, err)
		return
	}

	log.Printf("✅ Auto-invoicing: sent notice for project %d (%.2f óra)", project.ID, totalHours)
}
```

- [ ] **Step 2: Update the scheduler's call site**

In `backend/internal/services/scheduler.go`, change the doc comment and call:

```go
// StartAutoInvoiceScheduler runs RunAutoInvoiceNotices once on startup (in
// case the server was down at midnight on the 1st) and then every hour for
// as long as today is the 1st of the month. There is no other background job
// runner in this codebase, so a plain ticker is used rather than adding a
// cron dependency; RunAutoInvoiceNotices is idempotent per project/period, so
// firing it multiple times on the 1st is harmless.
func StartAutoInvoiceScheduler() {
	checkAndRun()

	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		checkAndRun()
	}
}

func checkAndRun() {
	if time.Now().Day() != 1 {
		return
	}
	log.Println("🗓️ Auto-invoicing: running monthly check")
	RunAutoInvoiceNotices()
}
```

- [ ] **Step 3: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: succeeds (confirms no other file still references `RunAutoInvoicing`, `autoInvoiceProject`, or `recordFailedAutoInvoice`).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/auto_invoice.go backend/internal/services/scheduler.go
git commit -m "$(cat <<'EOF'
feat(invoices): auto_invoice_enabled now sends notices, not invoices

Actual invoice creation is now always gated on an explicit approval
(manual button or notice-approval endpoint), regardless of this toggle.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: `InvoiceNoticeHandler.ApproveInvoiceNotice` + `services.SendInvoiceReadyEmail`

**Files:**
- Modify: `backend/internal/handlers/invoice_notice_handler.go`
- Modify: `backend/internal/services/invoice_notice.go`

**Interfaces:**
- Consumes: `createInvoiceForProject` (Task 3), `services.BillingoService.DownloadInvoicePDF(apiKey, billingoInvoiceID string) ([]byte, error)`, `models.BillingoSettings`, `checkInvoiceAccess`, `toInvoiceResponse`.
- Produces: `func (h *InvoiceNoticeHandler) ApproveInvoiceNotice(c *fiber.Ctx) error`; `func SendInvoiceReadyEmail(account models.GmailAccount, client models.Client, project models.Project, invoiceNumber string, pdfBytes []byte) error` — Task 8 registers the route for the handler method.

- [ ] **Step 1: Add `SendInvoiceReadyEmail` to `invoice_notice.go`**

Append to `backend/internal/services/invoice_notice.go`:

```go

// SendInvoiceReadyEmail notifies the client that their invoice has been
// issued, with the Billingo PDF attached. Sent once, immediately after
// InvoiceNoticeHandler.ApproveInvoiceNotice creates the invoice — never sent
// for invoices created via the manual "Számla kiállítása" button.
func SendInvoiceReadyEmail(account models.GmailAccount, client models.Client, project models.Project, invoiceNumber string, pdfBytes []byte) error {
	subject := fmt.Sprintf("Számla - %s", project.Name)
	body := fmt.Sprintf(
		"Kedves %s!\n\nMellékelten küldjük a(z) \"%s\" projekt %s számú számláját.\n\nÜdvözlettel",
		client.Name, project.Name, invoiceNumber,
	)
	attachments := []MessageAttachment{{
		Filename:    invoiceNumber + ".pdf",
		ContentType: "application/pdf",
		Data:        pdfBytes,
	}}
	raw := BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "", "", attachments)
	_, err := NewRealGmailAPI().SendMessage(context.Background(), &account, raw)
	return err
}
```

- [ ] **Step 2: Give `InvoiceNoticeHandler` a `billingoService`**

In `backend/internal/handlers/invoice_notice_handler.go`, change:

```go
type InvoiceNoticeHandler struct {
	permissionService *services.PermissionService
}

func NewInvoiceNoticeHandler() *InvoiceNoticeHandler {
	return &InvoiceNoticeHandler{permissionService: services.NewPermissionService()}
}
```

to:

```go
type InvoiceNoticeHandler struct {
	permissionService *services.PermissionService
	billingoService   *services.BillingoService
}

func NewInvoiceNoticeHandler() *InvoiceNoticeHandler {
	return &InvoiceNoticeHandler{
		permissionService: services.NewPermissionService(),
		billingoService:   services.NewBillingoService(),
	}
}
```

- [ ] **Step 3: Add `ApproveInvoiceNotice` at the end of the file, and add `"log"` to the import block**

Add `"log"` alongside the existing `"fmt"`-less import list (`"strconv"`, `"time"`, etc.) in `backend/internal/handlers/invoice_notice_handler.go`, then append:

```go

// ApproveInvoiceNotice - POST /api/v1/projects/:id/invoice-notices/:noticeId/approve
// Jóváhagyja a függőben lévő értesítőt: legyártja a tényleges Billingo
// számlát (createInvoiceForProject-tal, ugyanazzal a logikával mint a
// manuális "Számla kiállítása" gomb), majd e-mailben elküldi a PDF-et az
// ügyfélnek. Ha a PDF-küldés bármilyen okból meghiúsul, a számla attól még
// létrejön — csak egy figyelmeztető üzenetet kap vissza a jóváhagyó.
func (h *InvoiceNoticeHandler) ApproveInvoiceNotice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.create"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Invalid project id"})
	}
	noticeID, err := strconv.Atoi(c.Params("noticeId"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Invalid notice id"})
	}

	db := database.GetDB()

	var notice models.InvoiceNotice
	if err := db.Where("id = ? AND project_id = ?", noticeID, projectID).First(&notice).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Invoice notice not found"})
	}
	if notice.Status != "pending" {
		return c.Status(409).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Ez az értesítő már jóvá lett hagyva"})
	}

	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Project not found"})
	}
	if project.PricingType == "" {
		return c.Status(400).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Project has no pricing type configured"})
	}

	var client models.Client
	if err := db.First(&client, notice.ClientID).Error; err != nil {
		return c.Status(404).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: "Client not found"})
	}

	req := models.InvoiceCreateRequest{ClientID: notice.ClientID}
	if notice.PeriodStart != nil {
		req.PeriodStart = notice.PeriodStart.Format("2006-01-02")
	}
	if notice.PeriodEnd != nil {
		req.PeriodEnd = notice.PeriodEnd.Format("2006-01-02")
	}

	invoice, items, httpStatus, message := createInvoiceForProject(h.billingoService, project, client, req, currentUserID)
	if httpStatus != 0 {
		return c.Status(httpStatus).JSON(models.InvoiceNoticeApproveResponse{Success: false, Message: message})
	}

	now := time.Now()
	if err := db.Model(&notice).Updates(map[string]interface{}{
		"status":      "approved",
		"invoice_id":  invoice.ID,
		"approved_by": currentUserID,
		"approved_at": now,
	}).Error; err != nil {
		log.Printf("⚠️ Failed to mark invoice notice %d as approved: %v", notice.ID, err)
	}
	notice.Status = "approved"
	notice.InvoiceID = &invoice.ID
	notice.ApprovedBy = &currentUserID
	notice.ApprovedAt = &now

	emailSent := false
	warning := ""
	var account models.GmailAccount
	if err := db.Where("user_id = ?", currentUserID).First(&account).Error; err != nil {
		warning = "A számla elkészült, de nincs csatlakoztatott Gmail-fiókod — kérlek küldd el a PDF-et manuálisan."
	} else {
		var settings models.BillingoSettings
		if err := db.First(&settings, 1).Error; err != nil || settings.APIKey == "" {
			warning = "A számla elkészült, de a Billingo nincs beállítva a PDF letöltéséhez — kérlek küldd el manuálisan."
		} else {
			pdfBytes, err := h.billingoService.DownloadInvoicePDF(settings.APIKey, invoice.BillingoInvoiceID)
			if err != nil {
				log.Printf("⚠️ Failed to download PDF for invoice %d: %v", invoice.ID, err)
				warning = "A számla elkészült, de a PDF letöltése sikertelen — kérlek küldd el manuálisan."
			} else if err := services.SendInvoiceReadyEmail(account, client, project, invoice.BillingoInvoiceNumber, pdfBytes); err != nil {
				log.Printf("⚠️ Failed to send invoice-ready e-mail for invoice %d: %v", invoice.ID, err)
				warning = "A számla elkészült, de a PDF-es e-mail küldése sikertelen — kérlek küldd el manuálisan."
			} else {
				emailSent = true
			}
		}
	}

	response := toInvoiceResponse(*invoice, client.Name, "")
	response.Items = items

	return c.JSON(models.InvoiceNoticeApproveResponse{
		Success:   true,
		Message:   warning,
		Invoice:   &response,
		Notice:    &notice,
		EmailSent: emailSent,
	})
}
```

- [ ] **Step 4: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: succeeds.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/invoice_notice_handler.go backend/internal/services/invoice_notice.go
git commit -m "$(cat <<'EOF'
feat(invoices): add ApproveInvoiceNotice + SendInvoiceReadyEmail

Approving a pending notice creates the actual Billingo invoice via
createInvoiceForProject, then e-mails the client the PDF — only for
invoices created this way, never for the manual create-invoice button.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: `ListAllInvoiceNotices` + route registrations

**Files:**
- Modify: `backend/internal/handlers/invoice_notice_handler.go`
- Modify: `backend/internal/routes/invoice_routes.go`

**Interfaces:**
- Consumes: `models.InvoiceNoticeWithNames` (Task 2), `checkInvoiceAccess`.
- Produces: `GET /api/v1/invoice-notices?status=` → `{"success": true, "notices": []models.InvoiceNoticeWithNames}`; `POST /api/v1/projects/:id/invoice-notices/:noticeId/approve` → `ApproveInvoiceNotice` (Task 7) — Task 9's frontend `invoicesService.ts` calls both.

- [ ] **Step 1: Add `ListAllInvoiceNotices` to `invoice_notice_handler.go`**

Append at the end of the file:

```go

// ListAllInvoiceNotices - GET /api/v1/invoice-notices?status=pending - minden
// projekt értesítője, opcionális állapot-szűréssel, a Számlázás menüponthoz.
func (h *InvoiceNoticeHandler) ListAllInvoiceNotices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var rows []struct {
		models.InvoiceNotice
		ProjectName string `gorm:"column:project_name"`
		ClientName  string `gorm:"column:client_name"`
	}

	query := database.GetDB().Table("invoice_notices").
		Select("invoice_notices.*, projects.name as project_name, clients.name as client_name").
		Joins("LEFT JOIN projects ON invoice_notices.project_id = projects.id").
		Joins("LEFT JOIN clients ON invoice_notices.client_id = clients.id")

	if status := c.Query("status"); status != "" {
		query = query.Where("invoice_notices.status = ?", status)
	}

	if err := query.Order("invoice_notices.sent_at DESC").Scan(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error fetching invoice notices"})
	}

	notices := make([]models.InvoiceNoticeWithNames, 0, len(rows))
	for _, row := range rows {
		notices = append(notices, models.InvoiceNoticeWithNames{
			InvoiceNotice: row.InvoiceNotice,
			ProjectName:   row.ProjectName,
			ClientName:    row.ClientName,
		})
	}

	return c.JSON(fiber.Map{"success": true, "notices": notices})
}
```

- [ ] **Step 2: Register both routes**

In `backend/internal/routes/invoice_routes.go`, add right after the existing `projects.Get("/:id/invoice-notices", ...)` line:

```go
	// POST /api/v1/projects/:id/invoice-notices/:noticeId/approve - Értesítő jóváhagyása, számla létrehozása
	projects.Post("/:id/invoice-notices/:noticeId/approve", noticeHandler.ApproveInvoiceNotice)
```

and right after the existing `api.Get("/invoices", ...)` line at the bottom (before the closing `}`):

```go

	// GET /api/v1/invoice-notices - Minden értesítő listázása, opcionális ?status= szűréssel (Számlázás menüpont)
	api.Get("/invoice-notices", middleware.JWTMiddleware(), noticeHandler.ListAllInvoiceNotices)
```

- [ ] **Step 3: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: succeeds.

- [ ] **Step 4: Full backend test suite**

Run: `docker exec devbridge_backend go test ./...`
Expected: all pre-existing tests still pass, plus the two new `TestBuildInvoiceNoticeText*` tests from Task 4.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/invoice_notice_handler.go backend/internal/routes/invoice_routes.go
git commit -m "$(cat <<'EOF'
feat(invoices): add ListAllInvoiceNotices + approval/list routes

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: `invoicesService.ts` — approval + list-all client methods

**Files:**
- Modify: `frontend/src/services/invoicesService.ts`

**Interfaces:**
- Consumes: `GET /api/v1/invoice-notices?status=`, `POST /api/v1/projects/:id/invoice-notices/:noticeId/approve` (Task 8).
- Produces: extended `InvoiceNotice` TS interface (`status`, `invoice_id`, `approved_by`, `approved_at`); new `InvoiceNoticeWithNames` interface; `InvoiceNoticesService.approve(projectId, noticeId)`; `InvoiceNoticesService.listAll(status?)` — Task 10 (Billing page) and Task 11 (Invoice page) both import these.

- [ ] **Step 1: Extend the `InvoiceNotice` interface and add `InvoiceNoticeWithNames`**

In `frontend/src/services/invoicesService.ts`, replace:

```ts
export interface InvoiceNotice {
    id: number
    project_id: number
    client_id: number
    period_start: string | null
    period_end: string | null
    gmail_message_id: string
    sent_by: number
    sent_at: string
}
```

with:

```ts
export interface InvoiceNotice {
    id: number
    project_id: number
    client_id: number
    period_start: string | null
    period_end: string | null
    gmail_message_id: string
    sent_by: number
    sent_at: string
    status: 'pending' | 'approved'
    invoice_id: number | null
    approved_by: number | null
    approved_at: string | null
}

export interface InvoiceNoticeWithNames extends InvoiceNotice {
    project_name: string
    client_name: string
}
```

- [ ] **Step 2: Add `approve` and `listAll` to `InvoiceNoticesService`**

Replace the `InvoiceNoticesService` object with:

```ts
export const InvoiceNoticesService = {
    async send(projectId: number, payload: { client_id: number; period_start?: string; period_end?: string }) {
        return apiClient.post<{ success: boolean; message?: string; notice?: InvoiceNotice }>(
            `/projects/${projectId}/invoice-notice`,
            payload
        )
    },

    async list(projectId: number, periodStart?: string, periodEnd?: string) {
        return apiClient.get<{ success: boolean; notices?: InvoiceNotice[] }>(
            `/projects/${projectId}/invoice-notices`,
            { period_start: periodStart, period_end: periodEnd }
        )
    },

    async approve(projectId: number, noticeId: number) {
        return apiClient.post<{ success: boolean; message?: string; invoice?: Invoice; notice?: InvoiceNotice; email_sent: boolean }>(
            `/projects/${projectId}/invoice-notices/${noticeId}/approve`,
            {}
        )
    },

    async listAll(status?: 'pending' | 'approved') {
        return apiClient.get<{ success: boolean; notices?: InvoiceNoticeWithNames[] }>(
            '/invoice-notices',
            status ? { status } : undefined
        )
    },
}
```

- [ ] **Step 3: Typecheck**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors introduced by this file (any pre-existing unrelated errors are unaffected).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/invoicesService.ts
git commit -m "$(cat <<'EOF'
feat(invoices): add approve/listAll to InvoiceNoticesService

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Billing page — pending-notices approval section

**Files:**
- Modify: `frontend/src/app/dashboard/billing/page.tsx`

**Interfaces:**
- Consumes: `InvoiceNoticesService.listAll` and `.approve` (Task 9), `InvoiceNoticeWithNames`.
- Produces: a "Jóváhagyásra váró értesítők" section on the Billing page, and an updated `AutomationRow` label reflecting that the toggle now controls notice-sending only.

- [ ] **Step 1: Update the import line**

Change:

```ts
import { InvoicesService, Invoice, InvoiceLineItem } from '@/services/invoicesService';
```

to:

```ts
import { InvoicesService, Invoice, InvoiceLineItem, InvoiceNoticesService, InvoiceNoticeWithNames } from '@/services/invoicesService';
```

- [ ] **Step 2: Update `AutomationRow`'s label and helper copy**

In the `AutomationRow` component, change the checkbox label text:

```tsx
                Automatikus számlázás
```

to:

```tsx
                Automatikus havi értesítő
```

- [ ] **Step 3: Add pending-notices state and fetch to `BillingPage`**

In `BillingPage`, right after the existing `breakdownLoading` state declaration, add:

```ts
    const [pendingNotices, setPendingNotices] = React.useState<InvoiceNoticeWithNames[]>([]);
    const [loadingNotices, setLoadingNotices] = React.useState(true);
    const [approvingNoticeId, setApprovingNoticeId] = React.useState<number | null>(null);
    const [noticeApprovalError, setNoticeApprovalError] = React.useState<string | null>(null);

    const fetchPendingNotices = React.useCallback(() => {
        setLoadingNotices(true);
        InvoiceNoticesService.listAll('pending')
            .then((res) => setPendingNotices(res.notices || []))
            .catch(() => setPendingNotices([]))
            .finally(() => setLoadingNotices(false));
    }, []);

    React.useEffect(() => {
        fetchPendingNotices();
    }, [fetchPendingNotices]);

    const handleApproveNotice = async (notice: InvoiceNoticeWithNames) => {
        setNoticeApprovalError(null);
        try {
            setApprovingNoticeId(notice.id);
            const res = await InvoiceNoticesService.approve(notice.project_id, notice.id);
            if (!res.success && !res.notice) {
                setNoticeApprovalError(res.message || 'A jóváhagyás sikertelen');
                return;
            }
            if (res.message) {
                setNoticeApprovalError(res.message);
            }
            fetchPendingNotices();
            InvoicesService.getAllInvoices(selectedProjectId || undefined).then(setInvoices).catch(() => {});
        } catch (err: any) {
            setNoticeApprovalError(err.message);
        } finally {
            setApprovingNoticeId(null);
        }
    };
```

- [ ] **Step 4: Render the pending-notices section**

In the returned JSX, insert a new section right before the existing `<div>` that starts with `<h2 className="text-lg font-semibold text-foreground mb-3">Automatikus számlázás</h2>` (i.e. between the "Számlák" section and the "Automatikus számlázás" section):

```tsx
            <div>
                <h2 className="text-lg font-semibold text-foreground mb-3">Jóváhagyásra váró értesítők</h2>
                {noticeApprovalError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                        {noticeApprovalError}
                    </div>
                )}
                {loadingNotices ? (
                    <div className="flex items-center justify-center h-20">
                        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
                    </div>
                ) : pendingNotices.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs jóváhagyásra váró értesítő.</p>
                ) : (
                    <div className="bg-card border border-border rounded-lg divide-y divide-border">
                        {pendingNotices.map((notice) => (
                            <div key={notice.id} className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 px-4 py-3">
                                <div className="flex-1 min-w-0">
                                    <p className="text-sm font-medium text-foreground">{notice.project_name}</p>
                                    <p className="text-xs text-muted-foreground">
                                        {notice.client_name}
                                        {notice.period_start && notice.period_end ? ` · ${notice.period_start} – ${notice.period_end}` : ''}
                                        {' · elküldve: '}{new Date(notice.sent_at).toLocaleString('hu-HU')}
                                    </p>
                                </div>
                                <Button
                                    size="sm"
                                    loading={approvingNoticeId === notice.id}
                                    onClick={() => handleApproveNotice(notice)}
                                >
                                    Jóváhagyás
                                </Button>
                            </div>
                        ))}
                    </div>
                )}
            </div>
```

Make sure this new section's opening `<div>` is placed right before the existing `<div>` whose `<h2>` reads "Automatikus számlázás" — do not duplicate the outer wrapping `<div className="space-y-6">` (or equivalent) that already surrounds the page's sections; nest inside it like the other sections do.

- [ ] **Step 5: Typecheck**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors introduced by this file.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/app/dashboard/billing/page.tsx
git commit -m "$(cat <<'EOF'
feat(invoices): add pending-notices approval section to Billing page

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Invoice page — approve button + updated automation copy

**Files:**
- Modify: `frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx`

**Interfaces:**
- Consumes: `InvoiceNoticesService.approve` (Task 9), `InvoiceNotice` type (Task 9).
- Produces: final user-facing piece of the flow — no later task depends on this one.

- [ ] **Step 1: Import the `InvoiceNotice` type**

Change:

```ts
import { InvoicesService, Invoice, InvoiceExtraItemInput, InvoiceNoticesService } from '@/services/invoicesService';
```

to:

```ts
import { InvoicesService, Invoice, InvoiceExtraItemInput, InvoiceNoticesService, InvoiceNotice } from '@/services/invoicesService';
```

- [ ] **Step 2: Replace `noticeSentAt` with a full `notice` object so the approve button can read its id/status**

Change:

```ts
    const [noticeSending, setNoticeSending] = React.useState(false);
    const [noticeSentAt, setNoticeSentAt] = React.useState<string | null>(null);
    const [noticeError, setNoticeError] = React.useState<string | null>(null);
```

to:

```ts
    const [noticeSending, setNoticeSending] = React.useState(false);
    const [notice, setNotice] = React.useState<InvoiceNotice | null>(null);
    const [noticeError, setNoticeError] = React.useState<string | null>(null);
    const [approvingNotice, setApprovingNotice] = React.useState(false);
```

- [ ] **Step 3: Update the notice-fetching `useEffect` to keep the full notice object**

Change:

```ts
    React.useEffect(() => {
        if (!project || !selectedClientId) {
            setNoticeSentAt(null);
            return;
        }
        InvoiceNoticesService.list(project.id, periodStart || undefined, periodEnd || undefined)
            .then((res) => {
                const forClient = (res.notices || []).filter((n) => n.client_id === selectedClientId);
                setNoticeSentAt(forClient.length > 0 ? forClient[0].sent_at : null);
            })
            .catch(() => {
                // non-fatal: leave whatever noticeSentAt already holds
            });
    }, [project, selectedClientId, periodStart, periodEnd]);
```

to:

```ts
    React.useEffect(() => {
        if (!project || !selectedClientId) {
            setNotice(null);
            return;
        }
        InvoiceNoticesService.list(project.id, periodStart || undefined, periodEnd || undefined)
            .then((res) => {
                const forClient = (res.notices || []).filter((n) => n.client_id === selectedClientId);
                setNotice(forClient.length > 0 ? forClient[0] : null);
            })
            .catch(() => {
                // non-fatal: leave whatever notice already holds
            });
    }, [project, selectedClientId, periodStart, periodEnd]);
```

- [ ] **Step 4: Update `handleSendInvoiceNotice` to store the full notice**

Change:

```ts
            if (!res.success) {
                setNoticeError(res.message || 'Az értesítő küldése sikertelen');
                return;
            }
            setNoticeSentAt(res.notice?.sent_at || new Date().toISOString());
```

to:

```ts
            if (!res.success) {
                setNoticeError(res.message || 'Az értesítő küldése sikertelen');
                return;
            }
            setNotice(res.notice || null);
```

- [ ] **Step 5: Add `handleApproveNotice`, right after `handleSendInvoiceNotice`**

Insert after the closing `};` of `handleSendInvoiceNotice` (before `const handleSubmit = async () => {`):

```ts

    const handleApproveNotice = async () => {
        if (!project || !notice) return;
        setNoticeError(null);
        try {
            setApprovingNotice(true);
            const res = await InvoiceNoticesService.approve(project.id, notice.id);
            if (!res.success && !res.notice) {
                setNoticeError(res.message || 'A jóváhagyás sikertelen');
                return;
            }
            if (res.message) {
                setNoticeError(res.message);
            }
            if (res.notice) {
                setNotice(res.notice);
            }
            const refreshed = await InvoicesService.getProjectInvoices(project.id);
            setInvoices(refreshed);
        } catch (err: any) {
            setNoticeError(err.message);
        } finally {
            setApprovingNotice(false);
        }
    };
```

- [ ] **Step 6: Update the checkbox label and helper text**

Change:

```tsx
                        Automatikus havi számlázás
                        {savingAutoInvoice && <span className="text-xs text-muted-foreground font-normal">(mentés...)</span>}
                    </label>
                    <p className="text-xs text-muted-foreground mb-2">
                        Minden hónap 1-jén automatikusan kiszámlázza az előző havi órákat a kiválasztott ügyfélnek.
                    </p>
```

to:

```tsx
                        Automatikus havi értesítő
                        {savingAutoInvoice && <span className="text-xs text-muted-foreground font-normal">(mentés...)</span>}
                    </label>
                    <p className="text-xs text-muted-foreground mb-2">
                        Minden hónap 1-jén automatikusan e-mailt küld a kiválasztott ügyfélnek az előző havi órák alapján
                        tervezett számláról. A tényleges számla csak jóváhagyás után jön létre a Számlázás oldalon.
                    </p>
```

- [ ] **Step 7: Replace the notice-sent display block with one that also shows the approve button**

Change:

```tsx
                        <div className="mb-4">
                            <button
                                onClick={handleSendInvoiceNotice}
                                disabled={noticeSending || !selectedClientId}
                                className="px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 disabled:opacity-50 transition-colors"
                            >
                                {noticeSending ? 'Küldés...' : 'Értesítő küldése'}
                            </button>
                            {noticeSentAt && (
                                <p className="text-xs text-muted-foreground mt-1">
                                    Elküldve: {new Date(noticeSentAt).toLocaleString('hu-HU')}
                                </p>
                            )}
                            {noticeError && <p className="text-xs text-destructive mt-1">{noticeError}</p>}
                        </div>
```

to:

```tsx
                        <div className="mb-4">
                            <button
                                onClick={handleSendInvoiceNotice}
                                disabled={noticeSending || !selectedClientId}
                                className="px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 disabled:opacity-50 transition-colors"
                            >
                                {noticeSending ? 'Küldés...' : 'Értesítő küldése'}
                            </button>
                            {notice && (
                                <div className="mt-1 flex items-center gap-2">
                                    <p className="text-xs text-muted-foreground">
                                        Elküldve: {new Date(notice.sent_at).toLocaleString('hu-HU')}
                                        {notice.status === 'approved' ? ' · jóváhagyva' : ' · jóváhagyásra vár'}
                                    </p>
                                    {notice.status === 'pending' && (
                                        <Button size="sm" loading={approvingNotice} onClick={handleApproveNotice}>
                                            Jóváhagyás
                                        </Button>
                                    )}
                                </div>
                            )}
                            {noticeError && <p className="text-xs text-destructive mt-1">{noticeError}</p>}
                        </div>
```

- [ ] **Step 8: Typecheck**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors (confirms `noticeSentAt` was fully replaced, no dangling references).

- [ ] **Step 9: Lint**

Run: `docker exec devbridge_frontend npm run lint`
Expected: no new lint errors in this file.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx
git commit -m "$(cat <<'EOF'
feat(invoices): add in-app approve button to the invoice notice flow

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Final Verification (after all tasks)

- [ ] `docker exec devbridge_backend go build ./...`
- [ ] `docker exec devbridge_backend go vet ./...`
- [ ] `docker exec devbridge_backend go test ./...`
- [ ] `docker exec devbridge_frontend npx tsc --noEmit`
- [ ] `docker exec devbridge_frontend npm run lint`
- [ ] Manual, user-approved end-to-end check (NOT part of automated verification, since it mutates the real connected Billingo/Gmail accounts): send a real notice from the Invoice page, approve it from the Billing page, confirm a real Billingo invoice is created and the client receives the PDF e-mail; separately confirm the manual "Számla kiállítása" button still creates an invoice without sending any e-mail.

## Self-Review Notes

- **Spec coverage:** every requirement in `docs/superpowers/specs/2026-09-18-invoice-approval-flow-design.md` is covered — notice-sending automation via `auto_invoice_enabled` (Task 6), manual notice button unchanged in contract (Task 5), in-app-only approval with no client-facing approval link (Tasks 7-8, 10-11), manual "create invoice" button unaffected (Task 3 keeps it calling the same shared function with no e-mail side effect), invoice-ready e-mail sent only via the approval path (Task 7's `SendInvoiceReadyEmail` is called solely from `ApproveInvoiceNotice`, never from `CreateInvoice`), scheduler sends from `Project.CreatedBy`'s Gmail account (Task 6's `autoNotifyProject`).
- **Placeholder scan:** no TBD/TODO markers; every step carries complete code or an exact shell command.
- **Type consistency:** `models.InvoiceNoticeApproveResponse` (Task 2) fields (`Invoice`, `Notice`, `EmailSent`) match exactly what `ApproveInvoiceNotice` (Task 7) constructs and what the frontend `InvoiceNoticesService.approve` (Task 9) and its callers (Tasks 10-11) read; `createInvoiceForProject`'s signature (Task 3) matches its two call sites in Tasks 3 and 7 exactly; `InvoiceNoticeWithNames` (Task 2) matches `ListAllInvoiceNotices`'s construction (Task 8) and the Billing page's usage (Task 10).


