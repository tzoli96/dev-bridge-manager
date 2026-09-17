# Billingo Invoicing Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a project's fixed-price or hourly work be invoiced to Billingo (Hungarian e-invoicing SaaS) from the project page, using the already-implemented Client/pricing/time-tracking data.

**Architecture:** A new `invoice_handler.go` computes the billable amount from `Project.pricing_type`/`hourly_rate`/`fixed_price` (and, for hourly projects, `task_time_entries`), lazily creates/reuses a Billingo "partner" for the client, calls Billingo's document-create endpoint through a small `billingo_service.go` HTTP wrapper, and stores the result in a new `invoices` audit table. A new `billingo_settings` single-row table (admin-managed) holds the Billingo API key/block id. The frontend adds a "Számla kiállítása" button + invoice history on the project page, and a settings form under the existing admin "System Settings" card.

**Tech Stack:** Go/Fiber v2/GORM/PostgreSQL (backend, stdlib `net/http` for the new outbound Billingo client — no new dependency); Next.js/React/TypeScript (frontend), Tailwind, existing `apiClient` in `frontend/src/lib/api.ts`.

**Spec:** `docs/superpowers/specs/2026-09-17-billingo-invoicing-design.md`

## ⚠️ Execution note — read before starting Task 1

The repository currently has **~94 uncommitted files** in its working tree (verified via `git status --porcelain` during planning), including the entire recovered Client/ProjectClient backend and frontend Clients UI that this plan depends on, mixed together with unrelated dark-mode-redesign and task-attachments work. Task 1 below commits all of this as a single checkpoint commit — the repo already has precedent for this exact technique (`db85c63`, `f4c7d31`).

**This must happen directly in the main repository working directory, on whatever branch is currently checked out there — NOT inside an isolated git worktree.** If executing this plan via `superpowers:subagent-driven-development`, its Setup step creates an isolated worktree from the current committed `HEAD` before dispatching any task; a worktree created before Task 1's commit exists would not contain any of the 94 uncommitted files. **Task 1 must be executed inline, in the main repo, before that Setup step runs** (i.e., before `superpowers:using-git-worktrees` creates the worktree for Tasks 2–11). Only after Task 1's commit exists should a worktree/branch be created for the rest of this plan.

## Global Constraints

- Permissions introduced: `invoices.create`, `invoices.read`, `billingo_settings.manage`. Grants: `super_admin`/`admin` get all three; `manager` gets `invoices.create` + `invoices.read`; `user` gets `invoices.read` only.
- New migration is `000016` (latest existing is `000015_add_subtasks_and_done_column`). Include both `.up.sql` and `.down.sql`, matching the convention used by migrations `000011`–`000015` (not the older no-`.down.sql` convention from `000006`–`000010`).
- No new Go dependency: the Billingo HTTP client uses only `net/http`/`encoding/json` from the standard library.
- No new test-DB infrastructure: DB-touching code (handlers, `SumLoggedHours`, `EnsurePartner`/`CreateInvoice`) is verified via `docker exec devbridge_backend go build ./... && go vet ./...` plus manual `curl` smoke tests against the running dev stack — this repo has no `_test.go` files and no test-DB convention today. Only genuinely pure, DB-independent logic (`CalculateFixedAmount`, `CalculateHourlyAmount`, `BillingoService.doRequest`) gets real Go stdlib `testing` unit tests.
- Billingo's exact REST field/endpoint names are **not verified against live docs** (unreachable during planning). The drafted `billingo_service.go` code in Task 5 is a best-effort Billingo v3-style draft (`X-API-KEY` header, `/partners`, `/documents`, `should_send: false`) — Task 5's first step is to verify these against Billingo's current API documentation before treating them as final.
- VAT: Hungary's standard 27% rate (`"27%"` string) is sent on every invoice line item; no reverse-charge/exempt handling in this iteration (matches spec).
- `should_send`/auto-email is always `false` — Billingo must never email the invoice itself.
- Fixed-price projects can be invoiced at most once (checked via an existing `invoices` row with `status='created'`); the rejection message is the literal Hungarian string `"Ez a projekt már ki lett számlázva"`.
- Frontend invoice button/modal copy is Hungarian (`"Számla kiállítása"`); other new UI strings follow this codebase's existing convention of English labels with Hungarian only where the spec calls for it.
- `billingo_settings` is always exactly one row (`id=1`), seeded by the migration; `GET` never returns the plaintext API key (only `maskAPIKey` — asterisks + last 4 chars).

---

### Task 1: Checkpoint pre-existing uncommitted work

**Files:**
- No new files. Commits the ~94 currently modified/untracked files exactly as they stand.

**Interfaces:**
- Consumes: nothing.
- Produces: a clean working tree that Tasks 2–11 (and any worktree created for them) can branch from.

- [ ] **Step 1: Confirm current working directory is the main repo, not a worktree**

Run: `git rev-parse --show-toplevel && git rev-parse --git-dir`

Expected: both point at the main repo path (not a path containing `.worktrees/` or `worktrees/`). If this is already a worktree, stop — this task must run in the main repo.

- [ ] **Step 2: Review what will be committed**

Run: `git status --porcelain`

Expected: ~94 lines, matching modified/untracked files across `.docker/`, `backend/`, `frontend/`, and new `docs/superpowers/` files. No `.env`-style secret beyond `backend/.env` (already confirmed to contain only a non-sensitive CORS port change).

- [ ] **Step 3: Verify the tree still builds/typechecks before committing**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Run: `docker exec devbridge_frontend npx tsc --noEmit`

Expected: both exit 0 (already confirmed clean earlier in this session; re-run here as the final pre-commit gate).

- [ ] **Step 4: Commit everything as one checkpoint**

```bash
git add -A
git commit -m "chore: checkpoint pre-existing clients/pricing, attachments, and dark-mode WIP before Billingo invoicing work"
```

- [ ] **Step 5: Verify a clean tree**

Run: `git status --porcelain`
Expected: empty output.

---

### Task 2: Migration 000016 — invoices, billingo_settings, permissions

**Files:**
- Create: `backend/migrations/000016_create_invoices_and_billingo_settings.up.sql`
- Create: `backend/migrations/000016_create_invoices_and_billingo_settings.down.sql`

**Interfaces:**
- Consumes: existing `clients`, `projects`, `users`, `roles`, `permissions`, `role_permissions` tables.
- Produces: `invoices` table, `billingo_settings` table (seeded, `id=1`), `clients.billingo_partner_id` column, and the three new permissions with role grants — all later tasks' models/handlers assume these exist.

- [ ] **Step 1: Write the up migration**

```sql
-- backend/migrations/000016_create_invoices_and_billingo_settings.up.sql

ALTER TABLE clients ADD COLUMN billingo_partner_id VARCHAR(100);

CREATE TABLE invoices (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    billingo_invoice_id VARCHAR(50),
    billingo_invoice_number VARCHAR(50),
    pricing_type VARCHAR(20) NOT NULL,
    period_start DATE,
    period_end DATE,
    amount NUMERIC(12,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'created',
    error_message TEXT,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_invoices_project_id ON invoices(project_id);
CREATE INDEX idx_invoices_client_id ON invoices(client_id);
CREATE INDEX idx_invoices_status ON invoices(status);

CREATE TABLE billingo_settings (
    id SERIAL PRIMARY KEY,
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    block_id VARCHAR(50) NOT NULL DEFAULT '',
    updated_by INTEGER REFERENCES users(id),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO billingo_settings (id, api_key, block_id) VALUES (1, '', '');

-- Add invoices / billingo_settings permissions
INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('invoices.create', 'Create Invoices', 'Can create invoices for a project', 'invoices', 'create'),
    ('invoices.read', 'View Invoices', 'Can view a project''s invoice history', 'invoices', 'read'),
    ('billingo_settings.manage', 'Manage Billingo Settings', 'Can view and update the Billingo API key/settings', 'billingo_settings', 'manage');

-- Super Admin and Admin get all three
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin') AND p.name IN ('invoices.create', 'invoices.read', 'billingo_settings.manage');

-- Manager gets invoices.create and invoices.read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'manager' AND p.name IN ('invoices.create', 'invoices.read');

-- User gets invoices.read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'user' AND p.name = 'invoices.read';
```

- [ ] **Step 2: Write the down migration**

```sql
-- backend/migrations/000016_create_invoices_and_billingo_settings.down.sql

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE name IN ('invoices.create', 'invoices.read', 'billingo_settings.manage')
);
DELETE FROM permissions WHERE name IN ('invoices.create', 'invoices.read', 'billingo_settings.manage');

DROP TABLE IF EXISTS billingo_settings;
DROP TABLE IF EXISTS invoices;

ALTER TABLE clients DROP COLUMN billingo_partner_id;
```

- [ ] **Step 3: Apply and verify the migration**

Run: `docker compose -f .docker/docker-compose.yml restart backend` (or however this project's existing `RunMigrations` mechanism is triggered — matches the `2026-09-02` spec's own verification approach), then:

```bash
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "\d invoices"
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "\d billingo_settings"
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "SELECT * FROM billingo_settings;"
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "SELECT name FROM permissions WHERE name LIKE 'invoices.%' OR name = 'billingo_settings.manage';"
```

Expected: `invoices` and `billingo_settings` tables exist with the columns above; `billingo_settings` has exactly one row (`id=1`, empty `api_key`/`block_id`); all three new permission names are present.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/000016_create_invoices_and_billingo_settings.up.sql backend/migrations/000016_create_invoices_and_billingo_settings.down.sql
git commit -m "feat(billingo): add invoices and billingo_settings tables, permissions"
```

---

### Task 3: Go models — Invoice, BillingoSettings, Client.BillingoPartnerID

**Files:**
- Create: `backend/internal/models/invoice.go`
- Create: `backend/internal/models/billingo_settings.go`
- Modify: `backend/internal/models/client.go`

**Interfaces:**
- Consumes: `invoices`, `billingo_settings`, `clients.billingo_partner_id` from Task 2.
- Produces: `models.Invoice`, `models.InvoiceCreateRequest`, `models.InvoiceResponse`, `models.InvoiceListResponse`, `models.BillingoSettings`, `models.BillingoSettingsUpdateRequest`, `models.BillingoSettingsResponse` — consumed by Tasks 4, 5, 6, 7.

- [ ] **Step 1: Create the Invoice model**

```go
// backend/internal/models/invoice.go
package models

import "time"

type Invoice struct {
	ID                    uint       `json:"id" gorm:"primaryKey"`
	ProjectID             uint       `json:"project_id" gorm:"not null"`
	ClientID              uint       `json:"client_id" gorm:"not null"`
	BillingoInvoiceID     string     `json:"billingo_invoice_id" gorm:"size:50"`
	BillingoInvoiceNumber string     `json:"billingo_invoice_number" gorm:"size:50"`
	PricingType           string     `json:"pricing_type" gorm:"size:20"`
	PeriodStart           *time.Time `json:"period_start"`
	PeriodEnd             *time.Time `json:"period_end"`
	Amount                float64    `json:"amount"`
	Status                string     `json:"status" gorm:"default:created"`
	ErrorMessage          string     `json:"error_message" gorm:"type:text"`
	CreatedBy             uint       `json:"created_by" gorm:"not null"`
	CreatedAt             time.Time  `json:"created_at"`

	Client  Client `json:"client,omitempty" gorm:"foreignKey:ClientID"`
	Creator User   `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

type InvoiceCreateRequest struct {
	ClientID    uint   `json:"client_id" validate:"required"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type InvoiceResponse struct {
	ID                    uint       `json:"id"`
	ProjectID             uint       `json:"project_id"`
	ClientID              uint       `json:"client_id"`
	ClientName            string     `json:"client_name"`
	BillingoInvoiceID     string     `json:"billingo_invoice_id"`
	BillingoInvoiceNumber string     `json:"billingo_invoice_number"`
	PricingType           string     `json:"pricing_type"`
	PeriodStart           *time.Time `json:"period_start"`
	PeriodEnd             *time.Time `json:"period_end"`
	Amount                float64    `json:"amount"`
	Status                string     `json:"status"`
	ErrorMessage          string     `json:"error_message"`
	CreatedBy             uint       `json:"created_by"`
	CreatedByName         string     `json:"created_by_name"`
	CreatedAt             time.Time  `json:"created_at"`
}

type InvoiceListResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message"`
	Invoice  *InvoiceResponse  `json:"invoice,omitempty"`
	Invoices []InvoiceResponse `json:"invoices,omitempty"`
	Count    int               `json:"count,omitempty"`
}
```

- [ ] **Step 2: Create the BillingoSettings model**

```go
// backend/internal/models/billingo_settings.go
package models

import "time"

type BillingoSettings struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	APIKey    string    `json:"-" gorm:"column:api_key;size:255"`
	BlockID   string    `json:"block_id" gorm:"size:50"`
	UpdatedBy uint      `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BillingoSettings) TableName() string { return "billingo_settings" }

type BillingoSettingsUpdateRequest struct {
	APIKey  string `json:"api_key"`
	BlockID string `json:"block_id"`
}

type BillingoSettingsResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	APIKeyMasked string    `json:"api_key_masked"`
	BlockID      string    `json:"block_id"`
	UpdatedBy    uint      `json:"updated_by"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

- [ ] **Step 3: Add BillingoPartnerID to the Client model**

In `backend/internal/models/client.go`, add a field to the `Client` struct (after `Notes`):

```go
	Notes             string    `json:"notes" gorm:"type:text"`
	BillingoPartnerID string    `json:"billingo_partner_id" gorm:"column:billingo_partner_id;size:100"`
	IsActive          bool      `json:"is_active" gorm:"default:true"`
```

- [ ] **Step 4: Verify it compiles**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: exit 0, no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/invoice.go backend/internal/models/billingo_settings.go backend/internal/models/client.go
git commit -m "feat(billingo): add Invoice and BillingoSettings models, Client.BillingoPartnerID"
```

---

### Task 4: BillingoSettingsHandler + routes

**Files:**
- Create: `backend/internal/handlers/billingo_settings_handler.go`
- Create: `backend/internal/routes/billingo_settings_routes.go`

**Interfaces:**
- Consumes: `models.BillingoSettings`, `models.BillingoSettingsUpdateRequest`, `models.BillingoSettingsResponse` (Task 3); `services.NewPermissionService()`, `.CheckUserPermission(userID uint, permissionName string) (bool, error)`, `.GetUserWithPermissions(userID uint) (*models.User, error)` (existing `permission_service.go`); `database.GetDB() *gorm.DB` (existing).
- Produces: `handlers.NewBillingoSettingsHandler() *BillingoSettingsHandler`, `.GetBillingoSettings(c *fiber.Ctx) error`, `.UpdateBillingoSettings(c *fiber.Ctx) error`; `routes.SetupBillingoSettingsRoutes(api fiber.Router)` — wired into `routes.go` in Task 7.

- [ ] **Step 1: Write the handler**

```go
// backend/internal/handlers/billingo_settings_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type BillingoSettingsHandler struct {
	permissionService *services.PermissionService
}

func NewBillingoSettingsHandler() *BillingoSettingsHandler {
	return &BillingoSettingsHandler{permissionService: services.NewPermissionService()}
}

// checkBillingoSettingsAccess - jogosultság ellenőrzése, admin/super_admin fallback-kel
func (h *BillingoSettingsHandler) checkBillingoSettingsAccess(userID uint, permission string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, permission)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// GetBillingoSettings - GET /api/v1/admin/billingo-settings
func (h *BillingoSettingsHandler) GetBillingoSettings(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkBillingoSettingsAccess(currentUserID, "billingo_settings.manage"); err != nil {
		return err
	}

	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "Billingo settings not found",
		})
	}

	return c.JSON(models.BillingoSettingsResponse{
		Success:      true,
		Message:      "Billingo settings retrieved successfully",
		APIKeyMasked: maskAPIKey(settings.APIKey),
		BlockID:      settings.BlockID,
		UpdatedBy:    settings.UpdatedBy,
		UpdatedAt:    settings.UpdatedAt,
	})
}

// UpdateBillingoSettings - PUT /api/v1/admin/billingo-settings
func (h *BillingoSettingsHandler) UpdateBillingoSettings(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkBillingoSettingsAccess(currentUserID, "billingo_settings.manage"); err != nil {
		return err
	}

	var req models.BillingoSettingsUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if strings.TrimSpace(req.APIKey) == "" {
		return c.Status(400).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "API key is required",
		})
	}

	updates := map[string]interface{}{
		"api_key":    req.APIKey,
		"block_id":   req.BlockID,
		"updated_by": currentUserID,
	}

	if err := database.GetDB().Model(&models.BillingoSettings{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(models.BillingoSettingsResponse{
			Success: false,
			Message: "Error updating billingo settings",
		})
	}

	var settings models.BillingoSettings
	database.GetDB().First(&settings, 1)

	return c.JSON(models.BillingoSettingsResponse{
		Success:      true,
		Message:      "Billingo settings updated successfully",
		APIKeyMasked: maskAPIKey(settings.APIKey),
		BlockID:      settings.BlockID,
		UpdatedBy:    settings.UpdatedBy,
		UpdatedAt:    settings.UpdatedAt,
	})
}
```

- [ ] **Step 2: Write the routes**

```go
// backend/internal/routes/billingo_settings_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupBillingoSettingsRoutes(api fiber.Router) {
	billingoSettingsHandler := handlers.NewBillingoSettingsHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// GET /api/v1/admin/billingo-settings - Billingo beállítások lekérése (maszkolt API kulccsal)
	admin.Get("/billingo-settings", billingoSettingsHandler.GetBillingoSettings)

	// PUT /api/v1/admin/billingo-settings - Billingo beállítások frissítése
	admin.Put("/billingo-settings", billingoSettingsHandler.UpdateBillingoSettings)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: exit 0. (Route is not yet wired into `SetupRoutes` — that happens in Task 7 alongside invoice routes — so this only needs to compile standalone for now.)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/billingo_settings_handler.go backend/internal/routes/billingo_settings_routes.go
git commit -m "feat(billingo): add Billingo settings handler and routes"
```

---

### Task 5: Billingo API client (`billingo_service.go`)

**Files:**
- Create: `backend/internal/services/billingo_service.go`
- Test: `backend/internal/services/billingo_service_test.go`

**Interfaces:**
- Consumes: `models.Client` (`.BillingoPartnerID`, `.Name`, `.TaxNumber`, `.Email`, `.BillingZip`, `.BillingCity`, `.BillingAddress`), `models.BillingoSettings` (Task 3), `database.GetDB()`.
- Produces: `services.NewBillingoService() *BillingoService`, `.EnsurePartner(client *models.Client) (string, error)`, `.CreateInvoice(partnerID string, amount float64, description string) (invoiceID, invoiceNumber string, err error)` — consumed by Task 7's `InvoiceHandler`.

**⚠️ First, verify Billingo's actual API contract** (endpoint paths, auth header, partner/document JSON field names, VAT field format) against Billingo's current official API documentation before treating the payload/response shapes below as final — they are a best-effort v3-style draft written without doc access. Update the struct tags/paths/header name to match whatever the real docs say; the method signatures (`EnsurePartner`, `CreateInvoice`, `doRequest`) are the stable contract the rest of the app is built against and should not need to change.

- [ ] **Step 1: Write the failing test for `doRequest`**

```go
// backend/internal/services/billingo_service_test.go
package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoRequestSetsAuthHeaderAndParsesBody(t *testing.T) {
	var gotKey, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-KEY")
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer server.Close()

	svc := NewBillingoService()
	body, err := svc.doRequest(server.URL, "test-key", http.MethodPost, "/partners", map[string]string{"name": "Acme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKey != "test-key" {
		t.Fatalf("expected X-API-KEY header 'test-key', got %q", gotKey)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}

	var parsed map[string]int
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if parsed["id"] != 42 {
		t.Fatalf("expected id 42, got %d", parsed["id"])
	}
}

func TestDoRequestReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid api key"}`))
	}))
	defer server.Close()

	svc := NewBillingoService()
	_, err := svc.doRequest(server.URL, "bad-key", http.MethodGet, "/partners", nil)
	if err == nil {
		t.Fatal("expected an error for a 401 response, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestDoRequest -v`
Expected: FAIL — `NewBillingoService`/`doRequest` undefined.

- [ ] **Step 3: Implement `billingo_service.go`**

```go
// backend/internal/services/billingo_service.go
package services

import (
	"bytes"
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// billingoBaseURL is Billingo's production API base. See the file-level
// comment in the plan/spec: exact paths below are unverified against
// Billingo's live docs and must be confirmed before go-live.
const billingoBaseURL = "https://api.billingo.hu/v3"

type BillingoService struct {
	httpClient *http.Client
}

func NewBillingoService() *BillingoService {
	return &BillingoService{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *BillingoService) loadSettings() (*models.BillingoSettings, error) {
	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return nil, fmt.Errorf("billingo settings not configured: %w", err)
	}
	if settings.APIKey == "" {
		return nil, fmt.Errorf("billingo API key is not configured")
	}
	return &settings, nil
}

// doRequest sends an authenticated request to Billingo's API and returns the
// raw response body on 2xx. apiBaseURL is a parameter (not the billingoBaseURL
// constant) so tests can point it at an httptest.Server.
func (s *BillingoService) doRequest(apiBaseURL, apiKey, method, path string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding billingo request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, apiBaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("building billingo request: %w", err)
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling billingo: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading billingo response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("billingo returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type billingoPartnerAddress struct {
	CountryCode string `json:"country_code"`
	PostalCode  string `json:"postal_code"`
	City        string `json:"city"`
	Address     string `json:"address"`
}

type billingoPartnerPayload struct {
	Name    string                  `json:"name"`
	Emails  []string                `json:"emails,omitempty"`
	TaxCode string                  `json:"taxcode,omitempty"`
	Address billingoPartnerAddress  `json:"address"`
}

type billingoPartnerResponse struct {
	ID int `json:"id"`
}

// EnsurePartner returns the client's Billingo partner id, creating the
// partner in Billingo (and persisting the id on the client row) if none
// exists yet.
func (s *BillingoService) EnsurePartner(client *models.Client) (string, error) {
	if client.BillingoPartnerID != "" {
		return client.BillingoPartnerID, nil
	}

	settings, err := s.loadSettings()
	if err != nil {
		return "", err
	}

	payload := billingoPartnerPayload{
		Name:    client.Name,
		TaxCode: client.TaxNumber,
		Address: billingoPartnerAddress{
			CountryCode: "HU",
			PostalCode:  client.BillingZip,
			City:        client.BillingCity,
			Address:     client.BillingAddress,
		},
	}
	if client.Email != "" {
		payload.Emails = []string{client.Email}
	}

	respBody, err := s.doRequest(billingoBaseURL, settings.APIKey, http.MethodPost, "/partners", payload)
	if err != nil {
		return "", fmt.Errorf("creating billingo partner: %w", err)
	}

	var parsed billingoPartnerResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parsing billingo partner response: %w", err)
	}

	partnerID := fmt.Sprintf("%d", parsed.ID)
	if err := database.GetDB().Model(client).Update("billingo_partner_id", partnerID).Error; err != nil {
		return "", fmt.Errorf("saving billingo partner id: %w", err)
	}
	client.BillingoPartnerID = partnerID

	return partnerID, nil
}

type billingoInvoiceItemPayload struct {
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Vat       string  `json:"vat"`
}

type billingoInvoicePayload struct {
	PartnerID       int                          `json:"partner_id"`
	BlockID         int                          `json:"block_id"`
	Type            string                       `json:"type"`
	PaymentMethod   string                       `json:"payment_method"`
	Language        string                       `json:"language"`
	ShouldSendEmail bool                         `json:"should_send"`
	Items           []billingoInvoiceItemPayload `json:"items"`
}

type billingoInvoiceResponse struct {
	ID            int    `json:"id"`
	InvoiceNumber string `json:"invoice_number"`
}

// CreateInvoice finalizes (does not send/email) an invoice in Billingo for
// the given partner and net amount, applying Hungary's standard 27% VAT.
func (s *BillingoService) CreateInvoice(partnerID string, amount float64, description string) (invoiceID, invoiceNumber string, err error) {
	settings, err := s.loadSettings()
	if err != nil {
		return "", "", err
	}
	if settings.BlockID == "" {
		return "", "", fmt.Errorf("billingo block id is not configured")
	}

	var partnerIDInt, blockIDInt int
	if _, err := fmt.Sscanf(partnerID, "%d", &partnerIDInt); err != nil {
		return "", "", fmt.Errorf("invalid billingo partner id %q: %w", partnerID, err)
	}
	if _, err := fmt.Sscanf(settings.BlockID, "%d", &blockIDInt); err != nil {
		return "", "", fmt.Errorf("invalid billingo block id %q: %w", settings.BlockID, err)
	}

	payload := billingoInvoicePayload{
		PartnerID:       partnerIDInt,
		BlockID:         blockIDInt,
		Type:            "invoice",
		PaymentMethod:   "wire_transfer",
		Language:        "hu",
		ShouldSendEmail: false,
		Items: []billingoInvoiceItemPayload{
			{Name: description, Quantity: 1, UnitPrice: amount, Vat: "27%"},
		},
	}

	respBody, err := s.doRequest(billingoBaseURL, settings.APIKey, http.MethodPost, "/documents", payload)
	if err != nil {
		return "", "", fmt.Errorf("creating billingo invoice: %w", err)
	}

	var parsed billingoInvoiceResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", "", fmt.Errorf("parsing billingo invoice response: %w", err)
	}

	return fmt.Sprintf("%d", parsed.ID), parsed.InvoiceNumber, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestDoRequest -v`
Expected: PASS (both `TestDoRequestSetsAuthHeaderAndParsesBody` and `TestDoRequestReturnsErrorOnNon2xx`).

- [ ] **Step 5: Verify the whole backend still builds**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/billingo_service.go backend/internal/services/billingo_service_test.go
git commit -m "feat(billingo): add Billingo API client (EnsurePartner, CreateInvoice)"
```

---

### Task 6: Invoice amount calculation

**Files:**
- Create: `backend/internal/services/invoice_calc.go`
- Test: `backend/internal/services/invoice_calc_test.go`

**Interfaces:**
- Consumes: `database.GetDB()` (for `SumLoggedHours` only; `CalculateFixedAmount`/`CalculateHourlyAmount` are pure).
- Produces: `services.CalculateFixedAmount(fixedPrice float64) float64`, `services.CalculateHourlyAmount(totalHours, hourlyRate float64) float64`, `services.SumLoggedHours(projectID uint, periodStart, periodEnd time.Time) (float64, error)` — consumed by Task 7's `InvoiceHandler`.

- [ ] **Step 1: Write the failing tests for the pure calculation functions**

```go
// backend/internal/services/invoice_calc_test.go
package services

import "testing"

func TestCalculateFixedAmount(t *testing.T) {
	got := CalculateFixedAmount(150000)
	if got != 150000 {
		t.Fatalf("expected 150000, got %v", got)
	}
}

func TestCalculateHourlyAmount(t *testing.T) {
	got := CalculateHourlyAmount(12.5, 8000)
	want := 100000.0
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestCalculate -v`
Expected: FAIL — `CalculateFixedAmount`/`CalculateHourlyAmount` undefined.

- [ ] **Step 3: Implement `invoice_calc.go`**

```go
// backend/internal/services/invoice_calc.go
package services

import (
	"dev-bridge-manager/internal/database"
	"time"
)

// CalculateFixedAmount returns the full fixed price as the invoice amount.
func CalculateFixedAmount(fixedPrice float64) float64 {
	return fixedPrice
}

// CalculateHourlyAmount returns totalHours billed at hourlyRate.
func CalculateHourlyAmount(totalHours, hourlyRate float64) float64 {
	return totalHours * hourlyRate
}

// SumLoggedHours sums task_time_entries.hours for all entries whose task
// belongs to projectID and whose date falls within [periodStart, periodEnd].
func SumLoggedHours(projectID uint, periodStart, periodEnd time.Time) (float64, error) {
	var total float64
	err := database.GetDB().Table("task_time_entries").
		Select("COALESCE(SUM(task_time_entries.hours), 0)").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("tasks.project_id = ? AND task_time_entries.date BETWEEN ? AND ?", projectID, periodStart, periodEnd).
		Scan(&total).Error
	return total, err
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestCalculate -v`
Expected: PASS.

- [ ] **Step 5: Verify the whole backend still builds**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: exit 0. (`SumLoggedHours` is DB-dependent and gets a manual smoke test in Task 7's verification step, once `InvoiceHandler` actually calls it against real `task_time_entries` data.)

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/invoice_calc.go backend/internal/services/invoice_calc_test.go
git commit -m "feat(billingo): add invoice amount calculation helpers"
```

---

### Task 7: InvoiceHandler + routes + wire routes.go

**Files:**
- Create: `backend/internal/handlers/invoice_handler.go`
- Create: `backend/internal/routes/invoice_routes.go`
- Modify: `backend/internal/routes/routes.go`

**Interfaces:**
- Consumes: `models.Invoice`, `models.InvoiceCreateRequest`, `models.InvoiceResponse`, `models.InvoiceListResponse` (Task 3); `models.ProjectClient` (existing); `services.NewBillingoService()`, `.EnsurePartner`, `.CreateInvoice` (Task 5); `services.CalculateFixedAmount`, `services.CalculateHourlyAmount`, `services.SumLoggedHours` (Task 6); `services.NewPermissionService()` (existing).
- Produces: `handlers.NewInvoiceHandler() *InvoiceHandler`, `.CreateInvoice(c *fiber.Ctx) error`, `.GetProjectInvoices(c *fiber.Ctx) error`; `routes.SetupInvoiceRoutes(api fiber.Router)` — consumed by Task 8's frontend `invoicesService.ts` via the HTTP contract.

- [ ] **Step 1: Write the handler**

```go
// backend/internal/handlers/invoice_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

type InvoiceHandler struct {
	permissionService *services.PermissionService
	billingoService   *services.BillingoService
}

func NewInvoiceHandler() *InvoiceHandler {
	return &InvoiceHandler{
		permissionService: services.NewPermissionService(),
		billingoService:   services.NewBillingoService(),
	}
}

// checkInvoiceAccess - jogosultság ellenőrzése egy adott invoices.* jogra, admin/super_admin/manager fallback-kel
func (h *InvoiceHandler) checkInvoiceAccess(userID uint, action string) error {
	hasPermission, err := h.permissionService.CheckUserPermission(userID, action)
	if err != nil || !hasPermission {
		user, err := h.permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" && user.Role.Name != "manager" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}

func toInvoiceResponse(inv models.Invoice, clientName, createdByName string) models.InvoiceResponse {
	return models.InvoiceResponse{
		ID:                    inv.ID,
		ProjectID:             inv.ProjectID,
		ClientID:              inv.ClientID,
		ClientName:            clientName,
		BillingoInvoiceID:     inv.BillingoInvoiceID,
		BillingoInvoiceNumber: inv.BillingoInvoiceNumber,
		PricingType:           inv.PricingType,
		PeriodStart:           inv.PeriodStart,
		PeriodEnd:             inv.PeriodEnd,
		Amount:                inv.Amount,
		Status:                inv.Status,
		ErrorMessage:          inv.ErrorMessage,
		CreatedBy:             inv.CreatedBy,
		CreatedByName:         createdByName,
		CreatedAt:             inv.CreatedAt,
	}
}

func ptrInvoiceResponse(r models.InvoiceResponse) *models.InvoiceResponse { return &r }

// recordFailedInvoice persists an audit row for a Billingo call that failed
// after local validation already passed. A failed row never blocks a
// subsequent retry — only a status='created' row counts against the
// fixed-price once-only rule.
func (h *InvoiceHandler) recordFailedInvoice(projectID, clientID, createdBy uint, pricingType string, periodStart, periodEnd *time.Time, amount float64, errMsg string) {
	invoice := models.Invoice{
		ProjectID:    projectID,
		ClientID:     clientID,
		PricingType:  pricingType,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		Amount:       amount,
		Status:       "failed",
		ErrorMessage: errMsg,
		CreatedBy:    createdBy,
	}
	database.GetDB().Create(&invoice)
}

// CreateInvoice - POST /api/v1/projects/:id/invoices
func (h *InvoiceHandler) CreateInvoice(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	if err := h.checkInvoiceAccess(currentUserID, "invoices.create"); err != nil {
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

	var amount float64
	var periodStart, periodEnd *time.Time
	var description string

	switch project.PricingType {
	case "fixed":
		if project.FixedPrice == nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no fixed price configured"})
		}

		var existing models.Invoice
		if err := database.GetDB().Where("project_id = ? AND status = ?", projectID, "created").First(&existing).Error; err == nil {
			return c.Status(409).JSON(models.InvoiceListResponse{Success: false, Message: "Ez a projekt már ki lett számlázva"})
		}

		amount = services.CalculateFixedAmount(*project.FixedPrice)
		description = fmt.Sprintf("%s - fixed price", project.Name)

	case "hourly":
		if project.HourlyRate == nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Project has no hourly rate configured"})
		}
		if req.PeriodStart == "" || req.PeriodEnd == "" {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "period_start and period_end are required for hourly projects"})
		}

		start, err := time.Parse("2006-01-02", req.PeriodStart)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid period_start (expected YYYY-MM-DD)"})
		}
		end, err := time.Parse("2006-01-02", req.PeriodEnd)
		if err != nil {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid period_end (expected YYYY-MM-DD)"})
		}
		if start.After(end) {
			return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "period_start must not be after period_end"})
		}

		totalHours, err := services.SumLoggedHours(uint(projectID), start, end)
		if err != nil {
			return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Error summing logged hours"})
		}

		amount = services.CalculateHourlyAmount(totalHours, *project.HourlyRate)
		periodStart, periodEnd = &start, &end
		description = fmt.Sprintf("%s - %s to %s", project.Name, req.PeriodStart, req.PeriodEnd)

	default:
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Unsupported pricing type"})
	}

	partnerID, err := h.billingoService.EnsurePartner(&client)
	if err != nil {
		h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, err.Error())
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: "Billingo error: " + err.Error()})
	}

	billingoInvoiceID, billingoInvoiceNumber, err := h.billingoService.CreateInvoice(partnerID, amount, description)
	if err != nil {
		h.recordFailedInvoice(uint(projectID), req.ClientID, currentUserID, project.PricingType, periodStart, periodEnd, amount, err.Error())
		return c.Status(502).JSON(models.InvoiceListResponse{Success: false, Message: "Billingo error: " + err.Error()})
	}

	invoice := models.Invoice{
		ProjectID:             uint(projectID),
		ClientID:              req.ClientID,
		BillingoInvoiceID:     billingoInvoiceID,
		BillingoInvoiceNumber: billingoInvoiceNumber,
		PricingType:           project.PricingType,
		PeriodStart:           periodStart,
		PeriodEnd:             periodEnd,
		Amount:                amount,
		Status:                "created",
		CreatedBy:             currentUserID,
	}

	if err := database.GetDB().Create(&invoice).Error; err != nil {
		return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Invoice created in Billingo but failed to save locally"})
	}

	return c.Status(201).JSON(models.InvoiceListResponse{
		Success: true,
		Message: "Invoice created successfully",
		Invoice: ptrInvoiceResponse(toInvoiceResponse(invoice, client.Name, "")),
	})
}

// GetProjectInvoices - GET /api/v1/projects/:id/invoices
func (h *InvoiceHandler) GetProjectInvoices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := h.checkInvoiceAccess(currentUserID, "invoices.read"); err != nil {
		return err
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceListResponse{Success: false, Message: "Invalid project ID"})
	}

	var rows []struct {
		models.Invoice
		ClientName    string `gorm:"column:client_name"`
		CreatedByName string `gorm:"column:created_by_name"`
	}

	err = database.GetDB().Table("invoices").
		Select("invoices.*, clients.name as client_name, users.name as created_by_name").
		Joins("LEFT JOIN clients ON invoices.client_id = clients.id").
		Joins("LEFT JOIN users ON invoices.created_by = users.id").
		Where("invoices.project_id = ?", projectID).
		Order("invoices.created_at DESC").
		Scan(&rows).Error

	if err != nil {
		return c.Status(500).JSON(models.InvoiceListResponse{Success: false, Message: "Error fetching invoices"})
	}

	response := make([]models.InvoiceResponse, 0, len(rows))
	for _, row := range rows {
		response = append(response, toInvoiceResponse(row.Invoice, row.ClientName, row.CreatedByName))
	}

	return c.JSON(models.InvoiceListResponse{
		Success:  true,
		Message:  "Invoices retrieved successfully",
		Invoices: response,
		Count:    len(response),
	})
}
```

- [ ] **Step 2: Write the routes**

```go
// backend/internal/routes/invoice_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupInvoiceRoutes(api fiber.Router) {
	invoiceHandler := handlers.NewInvoiceHandler()

	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// POST /api/v1/projects/:id/invoices - Számla kiállítása egy projekthez
	projects.Post("/:id/invoices", invoiceHandler.CreateInvoice)

	// GET /api/v1/projects/:id/invoices - Projekt számla-előzményeinek listázása
	projects.Get("/:id/invoices", invoiceHandler.GetProjectInvoices)
}
```

- [ ] **Step 3: Wire both new route groups into `SetupRoutes`**

In `backend/internal/routes/routes.go`, change:

```go
	SetupClientRoutes(v1)            // Client endpoints
	SetupProjectClientRoutes(v1)     // Project-client assignment endpoints
```

to:

```go
	SetupClientRoutes(v1)            // Client endpoints
	SetupProjectClientRoutes(v1)     // Project-client assignment endpoints
	SetupInvoiceRoutes(v1)           // Invoice endpoints
	SetupBillingoSettingsRoutes(v1)  // Billingo settings endpoints
```

- [ ] **Step 4: Verify it compiles**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: exit 0.

- [ ] **Step 5: Manual smoke test against the running dev stack**

With the backend running and a valid admin JWT (`$TOKEN`) and an existing fixed-price project (`$PROJECT_ID`) that has a client attached (`$CLIENT_ID`), and Billingo settings populated with a real or sandbox API key + block id:

```bash
curl -s -X POST http://localhost:8080/api/v1/admin/billingo-settings \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"api_key":"<key>","block_id":"<block>"}'

curl -s http://localhost:8080/api/v1/admin/billingo-settings \
  -H "Authorization: Bearer $TOKEN"
# Expected: api_key_masked shows only last 4 chars, never the full key.

curl -s -X POST http://localhost:8080/api/v1/projects/$PROJECT_ID/invoices \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"client_id\": $CLIENT_ID}"
# Expected (fixed-price project): 201, success:true, an invoice with status "created".

curl -s -X POST http://localhost:8080/api/v1/projects/$PROJECT_ID/invoices \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"client_id\": $CLIENT_ID}"
# Expected (same fixed-price project, second call): 409, message "Ez a projekt már ki lett számlázva".

curl -s http://localhost:8080/api/v1/projects/$PROJECT_ID/invoices \
  -H "Authorization: Bearer $TOKEN"
# Expected: 200, invoices array containing the invoice created above.
```

If no Billingo sandbox key is available yet, expect the create call to fail with a 502 and a Billingo-error message (network/auth failure) rather than a panic or 500 — confirming the error path records a `failed` row without crashing. Verify with:

```bash
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "SELECT id, project_id, status, error_message FROM invoices ORDER BY id DESC LIMIT 1;"
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/invoice_handler.go backend/internal/routes/invoice_routes.go backend/internal/routes/routes.go
git commit -m "feat(billingo): add invoice creation/listing endpoints"
```

---

### Task 8: Frontend services — invoicesService.ts, billingoSettingsService.ts

**Files:**
- Create: `frontend/src/services/invoicesService.ts`
- Create: `frontend/src/services/billingoSettingsService.ts`

**Interfaces:**
- Consumes: `apiClient` from `frontend/src/lib/api.ts` (`get<T>`, `post<T>`, `put<T>`); the JSON shapes from Task 7 (`InvoiceListResponse`) and Task 4 (`BillingoSettingsResponse`).
- Produces: `InvoicesService.getProjectInvoices(projectId): Promise<Invoice[]>`, `InvoicesService.createInvoice(projectId, data): Promise<Invoice>`, `BillingoSettingsService.getSettings(): Promise<BillingoSettings>`, `BillingoSettingsService.updateSettings(data): Promise<BillingoSettings>` — consumed by Tasks 9, 10, 11.

- [ ] **Step 1: Write `invoicesService.ts`**

```typescript
// frontend/src/services/invoicesService.ts
import { apiClient } from '@/lib/api'

export interface Invoice {
    id: number
    project_id: number
    client_id: number
    client_name: string
    billingo_invoice_id: string
    billingo_invoice_number: string
    pricing_type: 'hourly' | 'fixed'
    period_start: string | null
    period_end: string | null
    amount: number
    status: 'created' | 'failed'
    error_message: string
    created_by: number
    created_by_name: string
    created_at: string
}

export interface InvoiceCreateRequest {
    client_id: number
    period_start?: string
    period_end?: string
}

export interface InvoicesResponse {
    success: boolean
    message: string
    invoice?: Invoice
    invoices?: Invoice[]
    count?: number
}

export class InvoicesService {
    private static baseUrl = '/projects'

    static async getProjectInvoices(projectId: number): Promise<Invoice[]> {
        try {
            const response = await apiClient.get<InvoicesResponse>(`${this.baseUrl}/${projectId}/invoices`)

            if (response.success) {
                return response.invoices || []
            }

            throw new Error(response.message || 'Failed to fetch invoices')
        } catch (error: any) {
            console.error('Error fetching invoices:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch invoices')
        }
    }

    static async createInvoice(projectId: number, data: InvoiceCreateRequest): Promise<Invoice> {
        try {
            const response = await apiClient.post<InvoicesResponse>(`${this.baseUrl}/${projectId}/invoices`, data)

            if (response.success && response.invoice) {
                return response.invoice
            }

            throw new Error(response.message || 'Failed to create invoice')
        } catch (error: any) {
            console.error('Error creating invoice:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to create invoice')
        }
    }
}
```

- [ ] **Step 2: Write `billingoSettingsService.ts`**

```typescript
// frontend/src/services/billingoSettingsService.ts
import { apiClient } from '@/lib/api'

export interface BillingoSettings {
    api_key_masked: string
    block_id: string
    updated_by: number
    updated_at: string
}

export interface BillingoSettingsUpdateRequest {
    api_key: string
    block_id?: string
}

export interface BillingoSettingsApiResponse {
    success: boolean
    message: string
    api_key_masked?: string
    block_id?: string
    updated_by?: number
    updated_at?: string
}

export class BillingoSettingsService {
    private static baseUrl = '/admin/billingo-settings'

    static async getSettings(): Promise<BillingoSettings> {
        try {
            const response = await apiClient.get<BillingoSettingsApiResponse>(this.baseUrl)

            if (response.success) {
                return {
                    api_key_masked: response.api_key_masked || '',
                    block_id: response.block_id || '',
                    updated_by: response.updated_by || 0,
                    updated_at: response.updated_at || ''
                }
            }

            throw new Error(response.message || 'Failed to fetch Billingo settings')
        } catch (error: any) {
            console.error('Error fetching Billingo settings:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch Billingo settings')
        }
    }

    static async updateSettings(data: BillingoSettingsUpdateRequest): Promise<BillingoSettings> {
        try {
            const response = await apiClient.put<BillingoSettingsApiResponse>(this.baseUrl, data)

            if (response.success) {
                return {
                    api_key_masked: response.api_key_masked || '',
                    block_id: response.block_id || '',
                    updated_by: response.updated_by || 0,
                    updated_at: response.updated_at || ''
                }
            }

            throw new Error(response.message || 'Failed to update Billingo settings')
        } catch (error: any) {
            console.error('Error updating Billingo settings:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to update Billingo settings')
        }
    }
}
```

- [ ] **Step 3: Verify it typechecks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/invoicesService.ts frontend/src/services/billingoSettingsService.ts
git commit -m "feat(billingo): add invoices and billingo settings frontend services"
```

---

### Task 9: CreateInvoiceModal.tsx

**Files:**
- Create: `frontend/src/components/CreateInvoiceModal.tsx`

**Interfaces:**
- Consumes: `Project`, `ProjectClient`, `ProjectsService.getProjectClients(projectId): Promise<ProjectClient[]>` (existing `projectsService.ts`); `InvoicesService.createInvoice`, `Invoice` (Task 8).
- Produces: `CreateInvoiceModal` component with props `{ isOpen: boolean, project: Project | null, onClose: () => void, onSuccess: (invoice: Invoice) => void }` — consumed by Task 10.

- [ ] **Step 1: Write the component**

```tsx
// frontend/src/components/CreateInvoiceModal.tsx
import { useState, useEffect } from 'react'
import { Project, ProjectClient, ProjectsService } from '@/services/projectsService'
import { InvoicesService, Invoice } from '@/services/invoicesService'

interface CreateInvoiceModalProps {
    isOpen: boolean
    project: Project | null
    onClose: () => void
    onSuccess: (invoice: Invoice) => void
}

export default function CreateInvoiceModal({ isOpen, project, onClose, onSuccess }: CreateInvoiceModalProps) {
    const [clients, setClients] = useState<ProjectClient[]>([])
    const [clientsLoading, setClientsLoading] = useState(true)
    const [selectedClientId, setSelectedClientId] = useState<number | null>(null)
    const [periodStart, setPeriodStart] = useState('')
    const [periodEnd, setPeriodEnd] = useState('')
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        if (isOpen && project) {
            setClientsLoading(true)
            ProjectsService.getProjectClients(project.id)
                .then(setClients)
                .catch(() => setClients([]))
                .finally(() => setClientsLoading(false))
        }
    }, [isOpen, project])

    const handleClose = () => {
        if (!loading) {
            setSelectedClientId(null)
            setPeriodStart('')
            setPeriodEnd('')
            setError(null)
            onClose()
        }
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!project) return

        if (!selectedClientId) {
            setError('Please select a client')
            return
        }

        if (project.pricing_type === 'hourly' && (!periodStart || !periodEnd)) {
            setError('Please select a period for hourly billing')
            return
        }

        try {
            setLoading(true)
            setError(null)

            const invoice = await InvoicesService.createInvoice(project.id, {
                client_id: selectedClientId,
                period_start: project.pricing_type === 'hourly' ? periodStart : undefined,
                period_end: project.pricing_type === 'hourly' ? periodEnd : undefined
            })

            onSuccess(invoice)
            handleClose()
        } catch (err: any) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    if (!isOpen || !project) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Számla kiállítása</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                    {error && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {error}
                        </div>
                    )}

                    <div>
                        <label htmlFor="client" className="block text-sm font-medium text-foreground mb-1">
                            Ügyfél *
                        </label>
                        <select
                            id="client"
                            value={selectedClientId ?? ''}
                            onChange={(e) => setSelectedClientId(e.target.value ? Number(e.target.value) : null)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                            disabled={loading || clientsLoading}
                            required
                        >
                            <option value="">
                                {clientsLoading ? 'Loading clients...' : 'Válasszon ügyfelet...'}
                            </option>
                            {clients.map((pc) => (
                                <option key={pc.client_id} value={pc.client_id}>
                                    {pc.client_name}
                                </option>
                            ))}
                        </select>
                        {!clientsLoading && clients.length === 0 && (
                            <p className="text-xs text-muted-foreground mt-1">
                                No clients are attached to this project yet.
                            </p>
                        )}
                    </div>

                    {project.pricing_type === 'hourly' && (
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label htmlFor="period_start" className="block text-sm font-medium text-foreground mb-1">
                                    Időszak kezdete *
                                </label>
                                <input
                                    type="date"
                                    id="period_start"
                                    value={periodStart}
                                    onChange={(e) => setPeriodStart(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                    required
                                />
                            </div>
                            <div>
                                <label htmlFor="period_end" className="block text-sm font-medium text-foreground mb-1">
                                    Időszak vége *
                                </label>
                                <input
                                    type="date"
                                    id="period_end"
                                    value={periodEnd}
                                    onChange={(e) => setPeriodEnd(e.target.value)}
                                    className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                    disabled={loading}
                                    required
                                />
                            </div>
                        </div>
                    )}

                    <div className="flex space-x-3 pt-4">
                        <button
                            type="button"
                            onClick={handleClose}
                            disabled={loading}
                            className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                        >
                            Mégse
                        </button>
                        <button
                            type="submit"
                            disabled={loading || clientsLoading || clients.length === 0}
                            className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                        >
                            {loading ? 'Kiállítás...' : 'Számla kiállítása'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}
```

- [ ] **Step 2: Verify it typechecks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/CreateInvoiceModal.tsx
git commit -m "feat(billingo): add CreateInvoiceModal component"
```

---

### Task 10: Wire pricing, invoice button, and history into the project page

**Files:**
- Modify: `frontend/src/app/dashboard/board/[projectId]/page.tsx`

**Interfaces:**
- Consumes: `ProjectsService.getProject(id): Promise<Project>` (existing); `InvoicesService.getProjectInvoices`, `Invoice` (Task 8); `CreateInvoiceModal` (Task 9); `useAuth` from `@/hooks/auth/use-auth` (existing adapter re-exporting `AuthContext`); `hasPermission` from `@/utils/permissions` (existing).
- Produces: updated `BoardsListPage` showing pricing info, a permission-gated "Számla kiállítása" button, and an invoice history list.

- [ ] **Step 1: Replace the file with the pricing/invoice-aware version**

```tsx
// frontend/src/app/dashboard/board/[projectId]/page.tsx
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { boardService } from '@/services/kanban';
import { ProjectsService, Project } from '@/services/projectsService';
import { InvoicesService, Invoice } from '@/services/invoicesService';
import { useAuth } from '@/hooks/auth/use-auth';
import { hasPermission } from '@/utils/permissions';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, ArrowLeft, KanbanSquare, FileText } from 'lucide-react';
import type { Board } from '@/types/kanban';
import CreateInvoiceModal from '@/components/CreateInvoiceModal';

export default function BoardsListPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { user } = useAuth();
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [project, setProject] = React.useState<Project | null>(null);
    const [invoices, setInvoices] = React.useState<Invoice[]>([]);
    const [isLoading, setIsLoading] = React.useState(true);
    const [newBoardName, setNewBoardName] = React.useState('');
    const [isCreating, setIsCreating] = React.useState(false);
    const [isInvoiceModalOpen, setIsInvoiceModalOpen] = React.useState(false);

    const loadProjectAndInvoices = React.useCallback(() => {
        ProjectsService.getProject(Number(projectId)).then((p) => {
            setProject(p);
            if (hasPermission(user, 'invoices.read')) {
                InvoicesService.getProjectInvoices(p.id).then(setInvoices).catch(() => setInvoices([]));
            }
        }).catch(() => setProject(null));
    }, [projectId, user]);

    React.useEffect(() => {
        boardService.listBoards(projectId)
            .then((data) => setBoards(data))
            .finally(() => setIsLoading(false));

        loadProjectAndInvoices();
    }, [projectId, loadProjectAndInvoices]);

    const handleCreate = async () => {
        if (!newBoardName.trim()) return;
        setIsCreating(true);
        try {
            const board = await boardService.createBoard(projectId, {
                name: newBoardName.trim(),
                position: boards.length,
            });
            setBoards((prev) => [...prev, board]);
            setNewBoardName('');
        } finally {
            setIsCreating(false);
        }
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

    return (
        <div className="p-6">
            <Link href="/dashboard/board" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
                <ArrowLeft size={14} /> Back to projects
            </Link>

            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold text-foreground">Boards</h1>
                {project?.pricing_type && hasPermission(user, 'invoices.create') && (
                    <Button icon={FileText} onClick={() => setIsInvoiceModalOpen(true)}>
                        Számla kiállítása
                    </Button>
                )}
            </div>

            {project?.pricing_type && (
                <div className="mb-6 p-4 bg-card border border-border rounded-lg">
                    <h2 className="text-sm font-medium text-foreground mb-1">Pricing</h2>
                    <p className="text-sm text-muted-foreground">
                        {project.pricing_type === 'hourly'
                            ? `Hourly — ${project.hourly_rate} HUF/hour`
                            : `Fixed price — ${project.fixed_price} HUF`}
                    </p>
                </div>
            )}

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                {boards.map((board) => (
                    <button
                        key={board.id}
                        onClick={() => router.push(`/dashboard/board/${projectId}/${board.id}`)}
                        className="flex items-center gap-3 p-4 bg-card border border-border rounded-lg shadow-sm hover:shadow-sm hover:border-primary/40 transition-all text-left"
                    >
                        <KanbanSquare className="text-primary" size={20} />
                        <span className="font-medium text-foreground">{board.name}</span>
                    </button>
                ))}
            </div>

            <div className="flex items-center gap-2 max-w-sm mb-6">
                <Input
                    value={newBoardName}
                    onChange={setNewBoardName}
                    placeholder="New board name..."
                    className="flex-1"
                />
                <Button icon={Plus} onClick={handleCreate} disabled={!newBoardName.trim()} loading={isCreating}>
                    New board
                </Button>
            </div>

            {hasPermission(user, 'invoices.read') && (
                <div>
                    <h2 className="text-lg font-semibold text-foreground mb-3">Invoice history</h2>
                    {invoices.length === 0 ? (
                        <p className="text-sm text-muted-foreground">No invoices created yet.</p>
                    ) : (
                        <div className="space-y-2">
                            {invoices.map((invoice) => (
                                <div key={invoice.id} className="flex items-center justify-between p-3 bg-card border border-border rounded-lg text-sm">
                                    <div>
                                        <div className="font-medium text-foreground">{invoice.billingo_invoice_number || '—'}</div>
                                        <div className="text-muted-foreground">
                                            {invoice.client_name} · {invoice.amount} HUF
                                            {invoice.period_start && invoice.period_end && ` · ${invoice.period_start} - ${invoice.period_end}`}
                                        </div>
                                    </div>
                                    <span className={invoice.status === 'created' ? 'text-success' : 'text-destructive'}>
                                        {invoice.status}
                                    </span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            )}

            <CreateInvoiceModal
                isOpen={isInvoiceModalOpen}
                project={project}
                onClose={() => setIsInvoiceModalOpen(false)}
                onSuccess={() => {
                    setIsInvoiceModalOpen(false);
                    loadProjectAndInvoices();
                }}
            />
        </div>
    );
}
```

- [ ] **Step 2: Verify it typechecks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 3: Manual in-browser verification**

Start the dev stack, log in as an admin/manager user, navigate to `/dashboard/board/<projectId>` for a project with `pricing_type` set and at least one attached client:
- Pricing info box shows the correct hourly/fixed value.
- "Számla kiállítása" button is visible; clicking opens `CreateInvoiceModal` with the project's attached clients in the dropdown (and a date-range picker only for hourly projects).
- After a successful create, the invoice history list refreshes and shows the new row.
- Log in as a `user`-role account: confirm the "Számla kiállítása" button is hidden but the invoice history list is still visible (per the `invoices.read` grant to `user`).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/dashboard/board/\[projectId\]/page.tsx
git commit -m "feat(billingo): wire pricing info, invoice creation, and invoice history into project page"
```

---

### Task 11: BillingoSettingsModal.tsx + wire into AdminTab.tsx

**Files:**
- Create: `frontend/src/components/BillingoSettingsModal.tsx`
- Modify: `frontend/src/components/dashboard/tabs/AdminTab.tsx`

**Interfaces:**
- Consumes: `BillingoSettingsService.getSettings`, `.updateSettings`, `BillingoSettings` (Task 8).
- Produces: `BillingoSettingsModal` component with props `{ isOpen: boolean, onClose: () => void }`; the existing "System Settings" admin card now opens it and is gated on `billingo_settings.manage` instead of the unused `system.settings` permission.

- [ ] **Step 1: Write the modal**

```tsx
// frontend/src/components/BillingoSettingsModal.tsx
import { useState, useEffect } from 'react'
import { BillingoSettingsService } from '@/services/billingoSettingsService'

interface BillingoSettingsModalProps {
    isOpen: boolean
    onClose: () => void
}

export default function BillingoSettingsModal({ isOpen, onClose }: BillingoSettingsModalProps) {
    const [apiKey, setApiKey] = useState('')
    const [blockId, setBlockId] = useState('')
    const [currentMaskedKey, setCurrentMaskedKey] = useState('')
    const [loading, setLoading] = useState(false)
    const [fetching, setFetching] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)

    useEffect(() => {
        if (isOpen) {
            setFetching(true)
            setSuccess(false)
            setError(null)
            BillingoSettingsService.getSettings()
                .then((settings) => {
                    setCurrentMaskedKey(settings.api_key_masked)
                    setBlockId(settings.block_id)
                })
                .catch((err) => setError(err.message))
                .finally(() => setFetching(false))
        }
    }, [isOpen])

    const handleClose = () => {
        if (!loading) {
            setApiKey('')
            setBlockId('')
            setError(null)
            setSuccess(false)
            onClose()
        }
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()

        if (!apiKey.trim()) {
            setError('API key is required to save')
            return
        }

        try {
            setLoading(true)
            setError(null)

            const updated = await BillingoSettingsService.updateSettings({
                api_key: apiKey.trim(),
                block_id: blockId.trim()
            })

            setCurrentMaskedKey(updated.api_key_masked)
            setApiKey('')
            setSuccess(true)
        } catch (err: any) {
            setError(err.message)
        } finally {
            setLoading(false)
        }
    }

    if (!isOpen) return null

    return (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 overflow-y-auto py-8">
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">Billingo Settings</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-muted-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                {fetching ? (
                    <div className="text-sm text-muted-foreground py-4">Loading...</div>
                ) : (
                    <form onSubmit={handleSubmit} className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {error}
                            </div>
                        )}
                        {success && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Settings saved successfully
                            </div>
                        )}

                        <div>
                            <label htmlFor="api_key" className="block text-sm font-medium text-foreground mb-1">
                                API Key {currentMaskedKey && `(current: ${currentMaskedKey})`}
                            </label>
                            <input
                                type="password"
                                id="api_key"
                                value={apiKey}
                                onChange={(e) => setApiKey(e.target.value)}
                                placeholder="Enter new API key to change it"
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="block_id" className="block text-sm font-medium text-foreground mb-1">
                                Block ID
                            </label>
                            <input
                                type="text"
                                id="block_id"
                                value={blockId}
                                onChange={(e) => setBlockId(e.target.value)}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div className="flex space-x-3 pt-4">
                            <button
                                type="button"
                                onClick={handleClose}
                                disabled={loading}
                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                            >
                                Close
                            </button>
                            <button
                                type="submit"
                                disabled={loading || !apiKey.trim()}
                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                            >
                                {loading ? 'Saving...' : 'Save'}
                            </button>
                        </div>
                    </form>
                )}
            </div>
        </div>
    )
}
```

- [ ] **Step 2: Wire it into `AdminTab.tsx`**

Replace the full contents of `frontend/src/components/dashboard/tabs/AdminTab.tsx`:

```tsx
'use client'

import { useState } from 'react'
import { User } from '@/types/user'
import { hasPermission } from '@/utils/permissions'
import BillingoSettingsModal from '@/components/BillingoSettingsModal'

interface AdminTabProps {
    user: User | null
}

export default function AdminTab({ user }: AdminTabProps) {
    const [isBillingoModalOpen, setIsBillingoModalOpen] = useState(false)

    const adminCards = [
        {
            title: "System Settings",
            description: "Configure system-wide settings",
            permission: "billingo_settings.manage",
            action: () => setIsBillingoModalOpen(true)
        },
        {
            title: "Role Management",
            description: "Manage user roles and permissions",
            permission: "roles.list",
            action: () => console.log("Manage Roles")
        },
        {
            title: "System Logs",
            description: "View system activity and logs",
            permission: "system.logs",
            action: () => console.log("View Logs")
        },
        {
            title: "Statistics",
            description: "System usage and performance metrics",
            permission: null, // Available for all admins
            action: () => console.log("View Stats")
        }
    ]

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-lg font-semibold text-foreground">Administration</h2>
                <p className="text-muted-foreground text-sm">System settings and administrative tools</p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {adminCards
                    .filter(card => !card.permission || hasPermission(user, card.permission))
                    .map(card => (
                        <div key={card.title} className="bg-card rounded-xl shadow-sm border border-border p-6 transition-shadow hover:shadow-sm">
                            <h3 className="font-medium text-foreground mb-1">{card.title}</h3>
                            <p className="text-muted-foreground text-sm mb-4">{card.description}</p>
                            <button
                                onClick={card.action}
                                className="bg-primary text-white px-4 py-2 rounded-lg font-medium text-sm shadow-sm hover:bg-primary/90 hover:shadow-sm active:scale-[0.98] transition-all"
                            >
                                Open {card.title.split(' ')[0]}
                            </button>
                        </div>
                    ))}
            </div>

            <BillingoSettingsModal
                isOpen={isBillingoModalOpen}
                onClose={() => setIsBillingoModalOpen(false)}
            />
        </div>
    )
}
```

- [ ] **Step 3: Verify it typechecks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 4: Manual in-browser verification**

Log in as `admin`/`super_admin`: the "System Settings" card is visible in the Admin tab; clicking it opens the Billingo settings modal, which loads the current masked key/block id, and saving a new key updates the masked display. Log in as a `manager` or `user` account (neither has `billingo_settings.manage`): confirm the "System Settings" card is no longer visible (previously it was gated on the unused `system.settings` permission, so this is an intentional narrowing).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/BillingoSettingsModal.tsx frontend/src/components/dashboard/tabs/AdminTab.tsx
git commit -m "feat(billingo): add Billingo settings modal, wire into admin System Settings card"
```

---

## Self-Review

**Spec coverage:**
- Client → Billingo partner reference: Task 3 (`billingo_partner_id`), Task 5 (`EnsurePartner`). ✅
- Local `invoices` audit table: Task 2, Task 3, Task 7. ✅
- `billingo_settings` single-row table, admin-managed: Task 2, Task 3, Task 4. ✅
- Invoice creation endpoint (amount calc, partner resolution, Billingo call, audit row): Task 6, Task 7. ✅
- Frontend invoice button + client/date picker, invoice history, settings form under "System Settings": Task 9, Task 10, Task 11. ✅
- Fixed-price once-only rule, hourly date-range summation: Task 7 (`CreateInvoice` switch cases). ✅
- Permissions/migration/grants: Task 2. ✅
- Error handling (failed rows, retry not blocked): Task 7 (`recordFailedInvoice`). ✅
- Non-goals (automatic monthly job, emailing, edit/cancel) are correctly absent from every task. ✅

**Placeholder scan:** No "TBD"/"similar to Task N"/unfilled error-handling language found in any step above; every code block is complete, and the one area of genuine external uncertainty (Billingo's exact field/endpoint names) is called out explicitly rather than hidden, in both the Global Constraints and Task 5's step-0 note.

**Type consistency:** `models.Invoice`/`InvoiceResponse`/`InvoiceListResponse` (Task 3) match the fields used in Task 7's handler and Task 8's TS interfaces (`billingo_invoice_number`, `period_start`/`period_end` as nullable, `status: 'created'|'failed'`). `services.CalculateFixedAmount`/`CalculateHourlyAmount`/`SumLoggedHours` signatures (Task 6) match their call sites in Task 7. `BillingoService.EnsurePartner`/`CreateInvoice` signatures (Task 5) match their call sites in Task 7. `BillingoSettingsService`/`InvoicesService` method names and return types (Task 8) match what Task 9, 10, 11 consume.

No gaps found; no fixes needed.
