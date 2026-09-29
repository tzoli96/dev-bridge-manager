# Project Password Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let every active member of a project store, view, edit, and
delete shared credentials (title, username, password, URL, notes) scoped
to that project, via a new "Jelszavak" page reached from the project
overview.

**Architecture:** A new `project_passwords` table holds AES-GCM-encrypted
passwords. A new `RequireProjectMember` middleware (built on the existing
`ProjectAssignment` model) gates all its routes to active project members
or app admins. A standard CRUD handler decrypts on read and encrypts on
write. The frontend adds a new route
`/dashboard/board/[projectId]/passwords` following this codebase's
existing project-sub-page pattern (`analytics`, `invoice`), built entirely
from already-existing UI components.

**Tech Stack:** Go/Fiber/GORM/PostgreSQL backend (existing), Next.js/React
frontend (existing), Go standard library `crypto/aes` + `crypto/cipher`
for encryption (new, no external dependency).

**Spec:** `docs/superpowers/specs/2026-09-29-project-password-manager-design.md`

## Global Constraints

- Passwords are AES-GCM encrypted at rest; the DB column is never
  plaintext (spec: "Data model", "Security").
- The encryption key comes from `os.Getenv("ENCRYPTION_KEY")`, falling
  back to a documented dev-only default — never fails startup (spec:
  "`crypto.go`").
- Every project member (any `ProjectAssignment.role` value) has equal,
  full CRUD rights; app `admin`/`super_admin` always bypass the
  membership check. No other role distinction (spec: confirmed "A").
- No audit log, no categories/tags, no per-entry permissions (spec:
  "Non-goals").
- All backend verification (`go build`, `go vet`, `go test`, `gofmt`) runs
  inside the `devbridge_backend` container; all frontend verification
  (`npx tsc --noEmit`, `npm run lint`) runs inside the `devbridge_frontend`
  container. Never on host.
- Migration number: `000041` (next available at spec time).
- Commit after every task. Do not push or open a PR — that's a separate,
  explicitly-requested step.

---

### Task 1: Database migration for `project_passwords`

**Files:**
- Create: `backend/migrations/000041_add_project_passwords.up.sql`
- Create: `backend/migrations/000041_add_project_passwords.down.sql`

**Interfaces:**
- Produces: the `project_passwords` table, consumed by Task 3's
  `models.ProjectPassword` (GORM's default pluralized-snake-case table
  name for that struct is `project_passwords`, so no `TableName()`
  override is needed).

- [ ] **Step 1: Write the up migration**

```sql
-- backend/migrations/000041_add_project_passwords.up.sql
CREATE TABLE project_passwords (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL DEFAULT '',
    encrypted_password TEXT NOT NULL,
    url VARCHAR(255) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    updated_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_project_passwords_project ON project_passwords (project_id);
```

- [ ] **Step 2: Write the down migration**

```sql
-- backend/migrations/000041_add_project_passwords.down.sql
DROP INDEX IF EXISTS idx_project_passwords_project;
DROP TABLE IF EXISTS project_passwords;
```

- [ ] **Step 3: Run the migration and verify**

```bash
docker restart devbridge_backend
sleep 3
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "\d project_passwords"
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "SELECT version, dirty FROM schema_migrations;"
```

Expected: `\d project_passwords` shows all columns from Step 1;
`schema_migrations` shows `version=41, dirty=false`.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/000041_add_project_passwords.up.sql backend/migrations/000041_add_project_passwords.down.sql
git commit -m "feat(passwords): add project_passwords table migration"
```

---

### Task 2: Encryption utility (`crypto.go`)

**Files:**
- Create: `backend/internal/services/crypto.go`
- Test: `backend/internal/services/crypto_test.go`

**Interfaces:**
- Produces: `services.Encrypt(plaintext string) (string, error)` and
  `services.Decrypt(ciphertext string) (string, error)`, consumed by
  Task 5's `ProjectPasswordHandler`.

- [ ] **Step 1: Write the failing tests**

```go
// backend/internal/services/crypto_test.go
package services

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	original := "hunter2"
	ciphertext, err := Encrypt(original)
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	if ciphertext == original {
		t.Fatal("expected ciphertext to differ from plaintext")
	}
	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("unexpected decrypt error: %v", err)
	}
	if decrypted != original {
		t.Fatalf("got %q, want %q", decrypted, original)
	}
}

func TestEncryptProducesDifferentCiphertextEachTime(t *testing.T) {
	a, err := Encrypt("same-password")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	b, err := Encrypt("same-password")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	if a == b {
		t.Fatal("expected two encryptions of the same plaintext to differ (random nonce)")
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	ciphertext, err := Encrypt("hunter2")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	tampered := ciphertext[:len(ciphertext)-4] + "abcd"
	if _, err := Decrypt(tampered); err == nil {
		t.Fatal("expected tampered ciphertext to fail to decrypt")
	}
}

func TestEncryptDecryptEmptyString(t *testing.T) {
	ciphertext, err := Encrypt("")
	if err != nil {
		t.Fatalf("unexpected encrypt error: %v", err)
	}
	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("unexpected decrypt error: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("got %q, want empty string", decrypted)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
docker exec devbridge_backend go test ./internal/services/... -run TestEncrypt -v
```

Expected: FAIL with "undefined: Encrypt" (the function doesn't exist
yet).

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/services/crypto.go
package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

// encryptionKey derives a 32-byte AES-256 key from ENCRYPTION_KEY, so any
// non-empty env value produces a valid key regardless of its length -
// mirroring auth_service.go's JWT_SECRET fallback pattern (falls back to
// a documented dev-only default rather than failing startup).
func encryptionKey() [32]byte {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		key = "default-encryption-key-change-in-production"
	}
	return sha256.Sum256([]byte(key))
}

// Encrypt returns base64(nonce || ciphertext) for plaintext, using
// AES-256-GCM with a fresh random nonce per call.
func Encrypt(plaintext string) (string, error) {
	key := encryptionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. It returns an error if ciphertext is
// malformed or its authentication tag doesn't verify (wrong key or
// tampered data).
func Decrypt(ciphertext string) (string, error) {
	key := encryptionKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
docker exec devbridge_backend go test ./internal/services/... -run TestEncrypt -v
docker exec devbridge_backend go test ./internal/services/... -run TestDecrypt -v
```

Expected: all four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/crypto.go backend/internal/services/crypto_test.go
git commit -m "feat(passwords): add AES-GCM encrypt/decrypt utility"
```

---

### Task 3: `ProjectPassword` model and request/DTO types

**Files:**
- Create: `backend/internal/models/project_password.go`

**Interfaces:**
- Consumes: none (pure data types).
- Produces: `models.ProjectPassword` (GORM entity),
  `models.ProjectPasswordCreateRequest`,
  `models.ProjectPasswordUpdateRequest`, `models.ProjectPasswordDTO` — all
  consumed by Task 5's handler.

- [ ] **Step 1: Write the model file**

```go
// backend/internal/models/project_password.go
package models

import "time"

type ProjectPassword struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	ProjectID         uint      `json:"project_id" gorm:"not null;index"`
	Title             string    `json:"title" gorm:"not null" validate:"required,min=1,max=255"`
	Username          string    `json:"username"`
	EncryptedPassword string    `json:"-" gorm:"column:encrypted_password;not null"`
	URL               string    `json:"url"`
	Notes             string    `json:"notes"`
	CreatedBy         uint      `json:"created_by" gorm:"not null"`
	UpdatedBy         uint      `json:"updated_by" gorm:"not null"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProjectPasswordCreateRequest struct {
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Username string `json:"username"`
	Password string `json:"password" validate:"required"`
	URL      string `json:"url"`
	Notes    string `json:"notes"`
}

type ProjectPasswordUpdateRequest struct {
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Username string `json:"username"`
	Password string `json:"password" validate:"required"`
	URL      string `json:"url"`
	Notes    string `json:"notes"`
}

// ProjectPasswordDTO is what the API actually returns: the password is
// decrypted server-side and included in the JSON body (the whole point
// of the feature is that a member can retrieve the real value), but it
// never lives on the ProjectPassword struct's exported JSON tag, so a
// stray `db.Find(&passwords)` handler elsewhere in the codebase can
// never leak it by accident - only buildPasswordDTO's explicit decrypt
// path can.
type ProjectPasswordDTO struct {
	ID            uint      `json:"id"`
	ProjectID     uint      `json:"project_id"`
	Title         string    `json:"title"`
	Username      string    `json:"username"`
	Password      string    `json:"password"`
	URL           string    `json:"url"`
	Notes         string    `json:"notes"`
	CreatedBy     uint      `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Verify it compiles**

```bash
docker exec devbridge_backend go build ./...
```

Expected: no output (clean build). There's no dedicated test for this
task — it's pure data types with no logic, exercised indirectly by
Task 2's tests (types compile) and Task 5's handler tests.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/project_password.go
git commit -m "feat(passwords): add ProjectPassword model and request/DTO types"
```

---

### Task 4: `RequireProjectMember` middleware

**Files:**
- Create: `backend/internal/middleware/project_member.go`
- Test: `backend/internal/middleware/project_member_test.go`

**Interfaces:**
- Consumes: `services.NewPermissionService().GetUserWithPermissions(userID) (*models.User, error)`
  (existing, returns a `*models.User` with `.Role.Name` populated —
  see `backend/internal/services/permission_service.go`);
  `models.ProjectAssignment` (existing,
  `backend/internal/models/project_assignment.go`).
- Produces: `middleware.RequireProjectMember() fiber.Handler`, consumed
  by Task 6's route registration.

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/middleware/project_member_test.go
package middleware

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIsProjectMemberAdminAlwaysQualifies(t *testing.T) {
	if !isProjectMember("admin", nil) {
		t.Error("expected admin with no assignment to qualify as a project member")
	}
	if !isProjectMember("super_admin", nil) {
		t.Error("expected super_admin with no assignment to qualify as a project member")
	}
}

func TestIsProjectMemberRequiresAssignment(t *testing.T) {
	if isProjectMember("user", nil) {
		t.Error("expected a non-admin user with no assignment to not qualify")
	}
	if !isProjectMember("user", &models.ProjectAssignment{Role: "viewer"}) {
		t.Error("expected a non-admin user with an active assignment to qualify, regardless of assignment role")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
docker exec devbridge_backend go test ./internal/middleware/... -run TestIsProjectMember -v
```

Expected: FAIL with "undefined: isProjectMember".

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/middleware/project_member.go
package middleware

import (
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

// isProjectMember reports whether a user with the given role name and
// (possibly nil) active project assignment should be treated as a
// member of a project: admins/super_admins always qualify; everyone
// else needs a non-nil assignment (any Role value on it - passwords
// give every active project member equal rights, per the approved
// design).
func isProjectMember(roleName string, assignment *models.ProjectAssignment) bool {
	if roleName == "admin" || roleName == "super_admin" {
		return true
	}
	return assignment != nil
}

// RequireProjectMember gates a /projects/:id/... route to users who are
// either an app-wide admin/super_admin, or have an active
// ProjectAssignment row for :id. Unlike RequirePermission/RequireRole,
// this checks membership in a *specific* project rather than a global
// role or permission - the first such check in this codebase (see the
// project password manager spec's "Important deviation" section).
func RequireProjectMember() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(uint)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"success": false, "message": "Authentication required"})
		}

		projectID, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
		}

		permissionService := services.NewPermissionService()
		user, err := permissionService.GetUserWithPermissions(userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to check project membership"})
		}

		var assignment *models.ProjectAssignment
		var found models.ProjectAssignment
		err = database.GetDB().
			Where("project_id = ? AND user_id = ? AND is_active = ?", projectID, userID, true).
			First(&found).Error
		if err == nil {
			assignment = &found
		}

		if !isProjectMember(user.Role.Name, assignment) {
			return c.Status(403).JSON(fiber.Map{"success": false, "message": "You are not a member of this project"})
		}

		return c.Next()
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
docker exec devbridge_backend go test ./internal/middleware/... -v
```

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/project_member.go backend/internal/middleware/project_member_test.go
git commit -m "feat(passwords): add RequireProjectMember middleware"
```

---

### Task 5: `ProjectPasswordHandler` (CRUD)

**Files:**
- Create: `backend/internal/handlers/project_password_handler.go`

**Interfaces:**
- Consumes: `services.Encrypt`/`services.Decrypt` (Task 2),
  `models.ProjectPassword`/`ProjectPasswordCreateRequest`/
  `ProjectPasswordUpdateRequest`/`ProjectPasswordDTO` (Task 3),
  `currentUserID(c *fiber.Ctx) uint` (existing package-level helper in
  `backend/internal/handlers/task_handler.go:19` - same `handlers`
  package, no import needed).
- Produces: `NewProjectPasswordHandler() *ProjectPasswordHandler` with
  `GetProjectPasswords`, `CreateProjectPassword`,
  `UpdateProjectPassword`, `DeleteProjectPassword` methods, consumed by
  Task 6's route registration.

- [ ] **Step 1: Write the handler**

```go
// backend/internal/handlers/project_password_handler.go
package handlers

import (
	"log"
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type ProjectPasswordHandler struct{}

func NewProjectPasswordHandler() *ProjectPasswordHandler {
	return &ProjectPasswordHandler{}
}

func buildPasswordDTO(p models.ProjectPassword, plaintext string) models.ProjectPasswordDTO {
	var createdByName string
	var user models.User
	if err := database.GetDB().Select("name").First(&user, p.CreatedBy).Error; err == nil {
		createdByName = user.Name
	}
	return models.ProjectPasswordDTO{
		ID:            p.ID,
		ProjectID:     p.ProjectID,
		Title:         p.Title,
		Username:      p.Username,
		Password:      plaintext,
		URL:           p.URL,
		Notes:         p.Notes,
		CreatedBy:     p.CreatedBy,
		CreatedByName: createdByName,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

// GetProjectPasswords - GET /api/v1/projects/:id/passwords
func (h *ProjectPasswordHandler) GetProjectPasswords(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var passwords []models.ProjectPassword
	if err := database.GetDB().Where("project_id = ?", projectID).Order("created_at DESC").Find(&passwords).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading passwords"})
	}

	dtos := make([]models.ProjectPasswordDTO, 0, len(passwords))
	for _, p := range passwords {
		plaintext, err := services.Decrypt(p.EncryptedPassword)
		if err != nil {
			log.Printf("project password %d: failed to decrypt: %v", p.ID, err)
			continue
		}
		dtos = append(dtos, buildPasswordDTO(p, plaintext))
	}

	return c.JSON(dtos)
}

// CreateProjectPassword - POST /api/v1/projects/:id/passwords
func (h *ProjectPasswordHandler) CreateProjectPassword(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.ProjectPasswordCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title and password are required"})
	}

	encrypted, err := services.Encrypt(req.Password)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error encrypting password"})
	}

	userID := currentUserID(c)
	password := models.ProjectPassword{
		ProjectID:         uint(projectID),
		Title:             req.Title,
		Username:          req.Username,
		EncryptedPassword: encrypted,
		URL:               req.URL,
		Notes:             req.Notes,
		CreatedBy:         userID,
		UpdatedBy:         userID,
	}
	if err := database.GetDB().Create(&password).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating password"})
	}

	return c.Status(201).JSON(buildPasswordDTO(password, req.Password))
}

// UpdateProjectPassword - PUT /api/v1/projects/:id/passwords/:passwordId
func (h *ProjectPasswordHandler) UpdateProjectPassword(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	passwordID, err := strconv.Atoi(c.Params("passwordId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid password ID"})
	}

	var password models.ProjectPassword
	if err := database.GetDB().Where("id = ? AND project_id = ?", passwordID, projectID).First(&password).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Password not found"})
	}

	var req models.ProjectPasswordUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title and password are required"})
	}

	encrypted, err := services.Encrypt(req.Password)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error encrypting password"})
	}

	password.Title = req.Title
	password.Username = req.Username
	password.EncryptedPassword = encrypted
	password.URL = req.URL
	password.Notes = req.Notes
	password.UpdatedBy = currentUserID(c)

	if err := database.GetDB().Save(&password).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating password"})
	}

	return c.JSON(buildPasswordDTO(password, req.Password))
}

// DeleteProjectPassword - DELETE /api/v1/projects/:id/passwords/:passwordId
func (h *ProjectPasswordHandler) DeleteProjectPassword(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}
	passwordID, err := strconv.Atoi(c.Params("passwordId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid password ID"})
	}

	result := database.GetDB().Where("id = ? AND project_id = ?", passwordID, projectID).Delete(&models.ProjectPassword{})
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting password"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Password not found"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Password deleted successfully"})
}
```

- [ ] **Step 2: Verify it compiles**

```bash
docker exec devbridge_backend go build ./...
docker exec devbridge_backend go vet ./...
```

Expected: both clean (no output).

There's no automated test for this task: this codebase has no
DB-backed handler test harness anywhere (confirmed during planning -
existing handler tests only cover pure, DB-free logic, e.g.
`activity_digest_handler_test.go`'s `digestDateRange`). Task 11 covers
this handler's behavior with real HTTP calls against the running
container instead, consistent with how the Jira integration's guards
were verified earlier in this project.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/project_password_handler.go
git commit -m "feat(passwords): add ProjectPasswordHandler CRUD endpoints"
```

---

### Task 6: Routes, route registration, and `ENCRYPTION_KEY` env var

**Files:**
- Create: `backend/internal/routes/project_password_routes.go`
- Modify: `backend/internal/routes/routes.go`
- Modify: `backend/.env`

**Interfaces:**
- Consumes: `handlers.NewProjectPasswordHandler()` (Task 5),
  `middleware.JWTMiddleware()` (existing),
  `middleware.RequireProjectMember()` (Task 4).
- Produces: the four live HTTP routes under `/api/v1/projects/:id/passwords`,
  consumed by Task 7's frontend service.

- [ ] **Step 1: Write the routes file**

```go
// backend/internal/routes/project_password_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProjectPasswordRoutes(api fiber.Router) {
	passwordHandler := handlers.NewProjectPasswordHandler()

	passwords := api.Group("/projects/:id/passwords")
	passwords.Use(middleware.JWTMiddleware())
	passwords.Use(middleware.RequireProjectMember())

	// GET /api/v1/projects/:id/passwords - Projekt jelszavainak listázása
	passwords.Get("/", passwordHandler.GetProjectPasswords)

	// POST /api/v1/projects/:id/passwords - Új jelszó létrehozása
	passwords.Post("/", passwordHandler.CreateProjectPassword)

	// PUT /api/v1/projects/:id/passwords/:passwordId - Jelszó frissítése
	passwords.Put("/:passwordId", passwordHandler.UpdateProjectPassword)

	// DELETE /api/v1/projects/:id/passwords/:passwordId - Jelszó törlése
	passwords.Delete("/:passwordId", passwordHandler.DeleteProjectPassword)
}
```

- [ ] **Step 2: Register it in `routes.go`**

In `backend/internal/routes/routes.go`, add one line after
`SetupClientStatusEmailRoutes(v1)`:

```go
	SetupClientStatusEmailRoutes(v1)     // AI-drafted weekly client status emails - super_admin only
	SetupProjectPasswordRoutes(v1)       // Project-scoped shared credentials - active project members only
}
```

- [ ] **Step 3: Add the `ENCRYPTION_KEY` placeholder to `.env`**

In `backend/.env`, add a new section after "JWT Configuration (later)":

```
# JWT Configuration (later)
# JWT_SECRET=your-super-secret-jwt-key
# JWT_EXPIRATION_HOURS=24

# Encryption Configuration (later)
# ENCRYPTION_KEY=your-super-secret-encryption-key
```

(Left commented, matching `JWT_SECRET`'s existing style: `crypto.go`'s
documented dev fallback covers local development without this being
set.)

- [ ] **Step 4: Restart the backend and verify the routes are live**

```bash
docker restart devbridge_backend
sleep 3
docker logs devbridge_backend --since 10s 2>&1 | tail -20
```

Expected: backend starts cleanly with no panic/error (route
registration only wires handlers, it doesn't touch the DB at startup).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/routes/project_password_routes.go backend/internal/routes/routes.go backend/.env
git commit -m "feat(passwords): wire project password routes and ENCRYPTION_KEY config"
```

---

### Task 7: Frontend `passwordsService.ts`

**Files:**
- Create: `frontend/src/services/passwordsService.ts`

**Interfaces:**
- Consumes: `apiClient` (existing, `frontend/src/lib/api.ts`).
- Produces: `passwordsService.list/create/update/delete`, consumed by
  Task 8/9's passwords page.

- [ ] **Step 1: Write the service**

```typescript
// frontend/src/services/passwordsService.ts
import { apiClient } from '@/lib/api';

export interface ProjectPassword {
    id: number;
    project_id: number;
    title: string;
    username: string;
    password: string;
    url: string;
    notes: string;
    created_by: number;
    created_by_name: string;
    created_at: string;
    updated_at: string;
}

export interface ProjectPasswordInput {
    title: string;
    username: string;
    password: string;
    url: string;
    notes: string;
}

export const passwordsService = {
    async list(projectId: string | number): Promise<ProjectPassword[]> {
        return apiClient.get(`/projects/${projectId}/passwords`);
    },

    async create(projectId: string | number, data: ProjectPasswordInput): Promise<ProjectPassword> {
        return apiClient.post(`/projects/${projectId}/passwords`, data);
    },

    async update(
        projectId: string | number,
        passwordId: number,
        data: ProjectPasswordInput
    ): Promise<ProjectPassword> {
        return apiClient.put(`/projects/${projectId}/passwords/${passwordId}`, data);
    },

    async remove(projectId: string | number, passwordId: number): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/passwords/${passwordId}`);
    },
};
```

- [ ] **Step 2: Verify it compiles**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: no new errors mentioning `passwordsService.ts` (pre-existing,
unrelated errors elsewhere in the project, if any, are out of scope -
see Task 11 for the full-project baseline check).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/services/passwordsService.ts
git commit -m "feat(passwords): add frontend passwordsService"
```

---

### Task 8: Passwords page - list, empty/loading/error states, create

**Files:**
- Create: `frontend/src/app/dashboard/board/[projectId]/passwords/page.tsx`

**Interfaces:**
- Consumes: `passwordsService` (Task 7), `useProject` (existing,
  `frontend/src/hooks/projects/use-project.ts`), `Modal`, `Input`,
  `Button`, `EmptyState`, `LoadingState`, `ErrorState` (all existing,
  `frontend/src/components/ui/`).
- Produces: the `/dashboard/board/[projectId]/passwords` route,
  consumed by Task 10's project-page button.

- [ ] **Step 1: Write the page**

```tsx
// frontend/src/app/dashboard/board/[projectId]/passwords/page.tsx
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useProject } from '@/hooks/projects/use-project';
import { passwordsService, ProjectPassword, ProjectPasswordInput } from '@/services/passwordsService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Modal } from '@/components/ui/modal';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { ArrowLeft, Plus, Eye, EyeOff, Copy, Pencil, Trash2, KeyRound } from 'lucide-react';

const emptyForm: ProjectPasswordInput = { title: '', username: '', password: '', url: '', notes: '' };

export default function ProjectPasswordsPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const { project, loading: projectLoading, error: projectError } = useProject(projectId);

    const [passwords, setPasswords] = React.useState<ProjectPassword[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [revealed, setRevealed] = React.useState<Record<number, boolean>>({});

    const [isModalOpen, setIsModalOpen] = React.useState(false);
    const [editingId, setEditingId] = React.useState<number | null>(null);
    const [form, setForm] = React.useState<ProjectPasswordInput>(emptyForm);
    const [isSaving, setIsSaving] = React.useState(false);

    const loadPasswords = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const data = await passwordsService.list(projectId);
            setPasswords(data);
        } catch (err: any) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    }, [projectId]);

    React.useEffect(() => {
        loadPasswords();
    }, [loadPasswords]);

    const openCreateModal = () => {
        setEditingId(null);
        setForm(emptyForm);
        setIsModalOpen(true);
    };

    const openEditModal = (entry: ProjectPassword) => {
        setEditingId(entry.id);
        setForm({
            title: entry.title,
            username: entry.username,
            password: entry.password,
            url: entry.url,
            notes: entry.notes,
        });
        setIsModalOpen(true);
    };

    const handleSave = async () => {
        if (!form.title.trim() || !form.password.trim()) return;
        setIsSaving(true);
        try {
            if (editingId) {
                const updated = await passwordsService.update(projectId, editingId, form);
                setPasswords((prev) => prev.map((p) => (p.id === editingId ? updated : p)));
            } else {
                const created = await passwordsService.create(projectId, form);
                setPasswords((prev) => [created, ...prev]);
            }
            setIsModalOpen(false);
        } catch (err: any) {
            setError(err.message);
        } finally {
            setIsSaving(false);
        }
    };

    const handleDelete = async (entry: ProjectPassword) => {
        if (!window.confirm(`Biztosan törlöd a(z) "${entry.title}" jelszót?`)) return;
        try {
            await passwordsService.remove(projectId, entry.id);
            setPasswords((prev) => prev.filter((p) => p.id !== entry.id));
        } catch (err: any) {
            setError(err.message);
        }
    };

    const toggleReveal = (id: number) => {
        setRevealed((prev) => ({ ...prev, [id]: !prev[id] }));
    };

    const copyToClipboard = async (value: string) => {
        await navigator.clipboard.writeText(value);
    };

    if (projectLoading) return <LoadingState message="Projekt betöltése..." />;
    if (projectError) return <ErrorState error={projectError} />;

    return (
        <div className="p-6 max-w-4xl mx-auto">
            <button
                onClick={() => router.push(`/dashboard/board/${projectId}`)}
                className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4"
            >
                <ArrowLeft size={14} /> Vissza a projekthez
            </button>

            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
                    <KeyRound size={22} /> Jelszavak{project ? ` — ${project.name}` : ''}
                </h1>
                <Button icon={Plus} onClick={openCreateModal}>
                    Új jelszó
                </Button>
            </div>

            {loading && <LoadingState message="Jelszavak betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={loadPasswords} />}

            {!loading && !error && passwords.length === 0 && (
                <EmptyState
                    icon="files"
                    title="Még nincs jelszó ehhez a projekthez"
                    description="Adj hozzá egy megosztott hitelesítő adatot (szerver, adatbázis, API kulcs), amit a projekt tagjai láthatnak."
                    action={{ label: 'Új jelszó', onClick: openCreateModal }}
                />
            )}

            {!loading && !error && passwords.length > 0 && (
                <div className="grid gap-4 sm:grid-cols-2">
                    {passwords.map((entry) => (
                        <div
                            key={entry.id}
                            className="bg-card border border-border rounded-lg p-4 shadow-sm hover:shadow-md transition-shadow"
                        >
                            <div className="flex items-start justify-between gap-2">
                                <div className="min-w-0">
                                    <h3 className="font-medium text-foreground truncate">{entry.title}</h3>
                                    {entry.url && (
                                        <a
                                            href={entry.url}
                                            target="_blank"
                                            rel="noreferrer"
                                            className="text-xs text-primary hover:underline break-all"
                                        >
                                            {entry.url}
                                        </a>
                                    )}
                                </div>
                                <div className="flex items-center gap-1 flex-shrink-0">
                                    <Button variant="ghost" size="icon-sm" icon={Pencil} onClick={() => openEditModal(entry)} />
                                    <Button variant="ghost" size="icon-sm" icon={Trash2} onClick={() => handleDelete(entry)} />
                                </div>
                            </div>

                            {entry.username && (
                                <div className="mt-3 flex items-center justify-between text-sm">
                                    <span className="text-muted-foreground truncate">{entry.username}</span>
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={Copy}
                                        onClick={() => copyToClipboard(entry.username)}
                                    />
                                </div>
                            )}

                            <div className="mt-1 flex items-center justify-between text-sm">
                                <span className="font-mono text-foreground">
                                    {revealed[entry.id] ? entry.password : '••••••••'}
                                </span>
                                <div className="flex items-center gap-1">
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={revealed[entry.id] ? EyeOff : Eye}
                                        onClick={() => toggleReveal(entry.id)}
                                    />
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={Copy}
                                        onClick={() => copyToClipboard(entry.password)}
                                    />
                                </div>
                            </div>

                            {entry.notes && <p className="mt-3 text-xs text-muted-foreground">{entry.notes}</p>}
                        </div>
                    ))}
                </div>
            )}

            <Modal
                isOpen={isModalOpen}
                onClose={() => setIsModalOpen(false)}
                title={editingId ? 'Jelszó szerkesztése' : 'Új jelszó'}
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    <Input label="Cím" value={form.title} onChange={(v) => setForm((f) => ({ ...f, title: v }))} />
                    <Input
                        label="Felhasználónév"
                        value={form.username}
                        onChange={(v) => setForm((f) => ({ ...f, username: v }))}
                    />
                    <Input
                        label="Jelszó"
                        value={form.password}
                        onChange={(v) => setForm((f) => ({ ...f, password: v }))}
                    />
                    <Input label="URL" value={form.url} onChange={(v) => setForm((f) => ({ ...f, url: v }))} />
                    <Input label="Jegyzet" value={form.notes} onChange={(v) => setForm((f) => ({ ...f, notes: v }))} />
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setIsModalOpen(false)}>
                            Mégse
                        </Button>
                        <Button
                            onClick={handleSave}
                            loading={isSaving}
                            disabled={!form.title.trim() || !form.password.trim()}
                        >
                            Mentés
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
}
```

- [ ] **Step 2: Verify it compiles**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: no new errors mentioning `passwords/page.tsx`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/dashboard/board/\[projectId\]/passwords/page.tsx
git commit -m "feat(passwords): add project passwords page with list, create, edit, delete, reveal, and copy"
```

---

### Task 9: Project overview page - "Jelszavak" button

**Files:**
- Modify: `frontend/src/app/dashboard/board/[projectId]/page.tsx`

**Interfaces:**
- Consumes: nothing new (navigates to Task 8's page via `router.push`).

- [ ] **Step 1: Add the `KeyRound` icon to the existing `lucide-react` import**

In `frontend/src/app/dashboard/board/[projectId]/page.tsx`, find:

```tsx
import { Plus, ArrowLeft, KanbanSquare, FileText, Eye, ChevronDown, ChevronUp, Receipt, ArrowUpRight, Wallet, CalendarClock, AlertOctagon, Repeat } from 'lucide-react';
```

Replace with:

```tsx
import { Plus, ArrowLeft, KanbanSquare, FileText, Eye, ChevronDown, ChevronUp, Receipt, ArrowUpRight, Wallet, CalendarClock, AlertOctagon, Repeat, KeyRound } from 'lucide-react';
```

- [ ] **Step 2: Add the button next to "Számla kiállítása"**

Find this block (around line 173-188):

```tsx
                <div className="flex items-center gap-2">
                    {hasPermission(user, 'invoices.read') && invoices.length > 0 && (
                        <Button
                            variant="secondary"
                            icon={Wallet}
                            onClick={() => router.push(`/dashboard/board/${projectId}/analytics`)}
                        >
                            Áttekintés
                        </Button>
                    )}
                    {(project?.pricing_type === 'hourly' || project?.pricing_type === 'fixed') && hasPermission(user, 'invoices.create') && (
                        <Button icon={FileText} onClick={() => router.push(`/dashboard/board/${projectId}/invoice`)}>
                            Számla kiállítása
                        </Button>
                    )}
                </div>
```

Replace with:

```tsx
                <div className="flex items-center gap-2">
                    {hasPermission(user, 'invoices.read') && invoices.length > 0 && (
                        <Button
                            variant="secondary"
                            icon={Wallet}
                            onClick={() => router.push(`/dashboard/board/${projectId}/analytics`)}
                        >
                            Áttekintés
                        </Button>
                    )}
                    <Button
                        variant="secondary"
                        icon={KeyRound}
                        onClick={() => router.push(`/dashboard/board/${projectId}/passwords`)}
                    >
                        Jelszavak
                    </Button>
                    {(project?.pricing_type === 'hourly' || project?.pricing_type === 'fixed') && hasPermission(user, 'invoices.create') && (
                        <Button icon={FileText} onClick={() => router.push(`/dashboard/board/${projectId}/invoice`)}>
                            Számla kiállítása
                        </Button>
                    )}
                </div>
```

The button has no `hasPermission` gate: per the approved design, every
active project member (and every admin/super_admin) has equal access,
so there's no narrower permission to check here - the backend's
`RequireProjectMember` is the actual enforcement point. A user without
project access who navigates here directly will see the 403
`ErrorState` from Task 8's page.

- [ ] **Step 3: Verify it compiles**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: no new errors mentioning this file.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/dashboard/board/\[projectId\]/page.tsx
git commit -m "feat(passwords): add Jelszavak button to project overview page"
```

---

### Task 10: Full verification pass

**Files:** none (verification only).

- [ ] **Step 1: Backend checks in the documented container**

```bash
docker exec devbridge_backend go build ./...
docker exec devbridge_backend go vet ./...
docker exec devbridge_backend go test ./...
docker exec devbridge_backend gofmt -l .
```

Expected: `build`/`vet` clean, `test` all PASS (including Task 2's and
Task 4's new tests), `gofmt -l .` reports only the pre-existing,
out-of-scope `internal/models/project.go` finding noted in this
project's prior Jira integration work (not something this plan
touches) - any other file listed is a regression to fix before
proceeding.

- [ ] **Step 2: Frontend checks in the documented container**

```bash
docker exec devbridge_frontend npx tsc --noEmit
docker exec devbridge_frontend npm run lint
```

Expected: no errors introduced by this plan's files (pre-existing,
unrelated errors elsewhere are out of scope - note them separately if
present, don't fix them here).

- [ ] **Step 3: Manual guard verification against the running stack**

Using an existing non-admin test user and project (or a fresh
throwaway user/project created via the app for this check, never real
production data):

```bash
# 1. As a user with NO ProjectAssignment on the target project: expect 403
curl -s -X GET "http://localhost:8080/api/v1/projects/<projectId>/passwords" \
  -H "Authorization: Bearer <non-member-token>"

# 2. As a project member (active ProjectAssignment row): expect 200 + []
curl -s -X GET "http://localhost:8080/api/v1/projects/<projectId>/passwords" \
  -H "Authorization: Bearer <member-token>"

# 3. Create as that member: expect 201 with the plaintext password echoed back
curl -s -X POST "http://localhost:8080/api/v1/projects/<projectId>/passwords" \
  -H "Authorization: Bearer <member-token>" -H "Content-Type: application/json" \
  -d '{"title":"Staging DB","username":"admin","password":"s3cr3t!","url":"","notes":""}'

# 4. Confirm at-rest encryption: the DB column must NOT contain the plaintext
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c \
  "SELECT title, encrypted_password FROM project_passwords ORDER BY id DESC LIMIT 1;"
```

Expected: step 1 returns 403 `"You are not a member of this project"`;
step 2 returns 200; step 3 returns 201 with `"password":"s3cr3t!"` in
the JSON body; step 4's `encrypted_password` column is base64 noise,
never the literal string `s3cr3t!`.

- [ ] **Step 4: Manual UI walkthrough in the browser**

Log in as a project member, navigate to the project, click "Jelszavak",
and confirm: empty state renders on a project with none yet; "Új
jelszó" creates an entry and it appears in the grid; the password
shows as `••••••••` by default and reveals on eye-icon click; the copy
buttons put the right value on the clipboard; edit updates an entry in
place; delete asks for confirmation and removes the entry; a direct
navigation to the page as a non-member renders the 403 `ErrorState`.

- [ ] **Step 5: Report results**

Summarize to the user which exact checks were run (Steps 1-4 above),
their pass/fail outcome, and explicitly separate any pre-existing,
out-of-scope findings (e.g. the known `project.go` gofmt issue) from
anything newly introduced by this plan.

No commit for this task (verification only, unless Step 1-4 surfaces a
bug to fix - in that case, fix it, add a regression check, and commit
the fix separately with its own message describing the bug).
