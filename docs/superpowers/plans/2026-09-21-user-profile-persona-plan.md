# User Profile / Persona Subsystem Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single, global "who am I" profile (background, expertise, tone rules, writing samples) that any AI-backed feature can draw on to write in the user's voice, with a super_admin-only admin UI to maintain it, and wire it into the first consumer: an "AI draft-reply" suggestion button on the Emails page.

**Architecture:** A singleton `Profile`/`ProfileSample` pair of GORM models (row `id=1`, golang-migrate managed) sits behind a `super_admin`-only admin CRUD endpoint. A pure `BuildProfileContext` service renders the profile into a prompt-ready text block, reused in-process by the new email draft-reply handler and exposed over `GET /api/v1/internal/profile-context` for other services on the docker-internal network. The AI service gets a new `/draft-reply` pydantic-ai endpoint mirroring the existing `/categorize-email` one. The Go backend calls it through a new `DraftReplyService`, mirroring the existing `EmailCategorizationService` HTTP-client pattern. The frontend gets a super_admin-only admin card + modal to edit the profile, and an "AI válasz-javaslat" button on the Emails page that pre-fills the reply editor.

**Tech Stack:** Go (Fiber, GORM, golang-migrate), Python (FastAPI, pydantic-ai, Gemini via `google:<model>`), Next.js/React (TypeScript, tiptap), PostgreSQL.

**Spec:** `docs/superpowers/specs/2026-09-21-user-profile-persona-design.md`

## Global Constraints

- Profile access is gated to the `super_admin` role **only** — no fallback to `admin`, unlike every other admin screen in this codebase. Enforced via route-level `middleware.RequireRole("super_admin")`, never a group-wide `.Use()` (see Task 3 for why).
- Single global singleton profile, row `id=1`. No per-user profiles, no versioning/audit history.
- The AI never sends anything automatically. The draft-reply endpoint only returns text; a human always reviews and manually sends.
- Schema changes go exclusively through golang-migrate files under `backend/migrations/`; no GORM `AutoMigrate` anywhere.
- Before creating `backend/migrations/000030_add_profile.up/down.sql`, run `ls backend/migrations | sort -V | tail -5` to confirm `000030` is still free — other in-flight work may have claimed it since this plan was written.
- No new test infrastructure: this codebase has zero DB-backed/integration tests. Pure functions (like `BuildProfileContext`, HTTP-client services against `httptest.Server`) get unit tests; DB-touching Fiber handlers do not, matching the existing convention (`email_handler_test.go` only tests the one pure helper, `isValidEmailCategory`).
- AI service model config: each pydantic-ai `Agent` module reads its own `GEMINI_MODEL` env var independently (default `gemini-2.5-flash`) rather than importing it from another agent module, to avoid circular imports — same reasoning as the existing comment in `ai/app/categorize_email.py`.
- Run all backend Go commands via `docker exec devbridge_backend ...`, all AI-service Python commands via `docker exec devbridge_ai ...`, and all frontend commands via `docker exec devbridge_frontend ...`. Never run them on the host.

---

### Task 1: Profile & ProfileSample models + migration

**Files:**
- Create: `backend/internal/models/profile.go`
- Create: `backend/migrations/000030_add_profile.up.sql`
- Create: `backend/migrations/000030_add_profile.down.sql`

**Interfaces:**
- Produces: `models.Profile{ID, Background, Expertise, ToneRules, Samples []ProfileSample, UpdatedBy uint, UpdatedAt time.Time}` (table `profiles`, singleton row `id=1`, seeded by the migration). `models.ProfileSample{ID, ProfileID, Label, Content}` (table `profile_samples`, FK `profile_id`). `models.ProfileSampleInput{Label, Content}` and `models.ProfileUpdateRequest{Background, Expertise, ToneRules, Samples []ProfileSampleInput}` for the admin PUT body. `models.ProfileResponse{Success bool, Message string, Profile *Profile}` for handler responses. These exact names/fields are consumed by Task 2 (`BuildProfileContext(profile *models.Profile)`) and Task 3 (`ProfileHandler`).

There is no existing test file for any model in this codebase (`find backend/internal/models -iname "*_test.go"` returns nothing) — models here are plain structs with no testable logic, so this task has no unit test. Verification is: the migration runs cleanly and the code compiles.

- [ ] **Step 1: Confirm the next-free migration number**

Run: `ls backend/migrations | sort -V | tail -5`
Expected: highest existing pair is `000029_add_email_category.up/down.sql`. If a `000030` pair already exists from other work, use the next free number instead and adjust the filenames in the remaining steps accordingly.

- [ ] **Step 2: Write the migration**

`backend/migrations/000030_add_profile.up.sql`:

```sql
-- backend/migrations/000030_add_profile.up.sql

CREATE TABLE profiles (
    id SERIAL PRIMARY KEY,
    background TEXT NOT NULL DEFAULT '',
    expertise TEXT NOT NULL DEFAULT '',
    tone_rules TEXT NOT NULL DEFAULT '',
    updated_by INTEGER REFERENCES users(id),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO profiles (id, background, expertise, tone_rules) VALUES (1, '', '', '');

CREATE TABLE profile_samples (
    id SERIAL PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_profile_samples_profile_id ON profile_samples(profile_id);
```

`backend/migrations/000030_add_profile.down.sql`:

```sql
-- backend/migrations/000030_add_profile.down.sql

DROP TABLE profile_samples;
DROP TABLE profiles;
```

- [ ] **Step 3: Write the model**

`backend/internal/models/profile.go`:

```go
// backend/internal/models/profile.go
package models

import "time"

type Profile struct {
	ID         uint            `json:"id" gorm:"primaryKey"`
	Background string          `json:"background" gorm:"type:text"`
	Expertise  string          `json:"expertise" gorm:"type:text"`
	ToneRules  string          `json:"tone_rules" gorm:"column:tone_rules;type:text"`
	Samples    []ProfileSample `json:"samples" gorm:"foreignKey:ProfileID"`
	UpdatedBy  uint            `json:"updated_by"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func (Profile) TableName() string { return "profiles" }

type ProfileSample struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ProfileID uint   `json:"profile_id" gorm:"not null;index"`
	Label     string `json:"label" gorm:"size:100"`
	Content   string `json:"content" gorm:"type:text"`
}

func (ProfileSample) TableName() string { return "profile_samples" }

type ProfileSampleInput struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

type ProfileUpdateRequest struct {
	Background string               `json:"background"`
	Expertise  string               `json:"expertise"`
	ToneRules  string               `json:"tone_rules"`
	Samples    []ProfileSampleInput `json:"samples"`
}

type ProfileResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message,omitempty"`
	Profile *Profile `json:"profile,omitempty"`
}
```

- [ ] **Step 4: Build and run the migration**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds with no errors.

Restart the backend container so `RunMigrations` (called at startup, `backend/internal/database/migrate.go`) picks up the new file:

Run: `docker restart devbridge_backend`

Then check the migration applied and the singleton row exists:

Run: `docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "SELECT id, background, expertise, tone_rules FROM profiles;"`
Expected: one row, `id=1`, all three text columns empty strings.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/profile.go backend/migrations/000030_add_profile.up.sql backend/migrations/000030_add_profile.down.sql
git commit -m "feat(profile): add Profile/ProfileSample models and migration"
```

---

### Task 2: profile_context.go builder service

**Files:**
- Create: `backend/internal/services/profile_context.go`
- Test: `backend/internal/services/profile_context_test.go`

**Interfaces:**
- Consumes: `models.Profile{Background, Expertise, ToneRules, Samples []models.ProfileSample}` from Task 1.
- Produces: `services.BuildProfileContext(profile *models.Profile) string` — a pure function, no DB access. Returns `""` for an empty/nil profile. Consumed by Task 3's `GetProfileContext` handler and Task 5's `EmailHandler.DraftReply` handler.

- [ ] **Step 1: Write the failing tests**

`backend/internal/services/profile_context_test.go`:

```go
// backend/internal/services/profile_context_test.go
package services

import (
	"dev-bridge-manager/internal/models"
	"strings"
	"testing"
)

func TestBuildProfileContextEmptyProfileReturnsEmptyString(t *testing.T) {
	profile := &models.Profile{}
	if got := BuildProfileContext(profile); got != "" {
		t.Fatalf("expected empty string for empty profile, got %q", got)
	}
}

func TestBuildProfileContextNilProfileReturnsEmptyString(t *testing.T) {
	if got := BuildProfileContext(nil); got != "" {
		t.Fatalf("expected empty string for nil profile, got %q", got)
	}
}

func TestBuildProfileContextIncludesAllFilledSections(t *testing.T) {
	profile := &models.Profile{
		Background: "Fejlesztő és vállalkozó",
		Expertise:  "Backend rendszerek, Go, Python",
		ToneRules:  "Közvetlen, tegeződő, rövid mondatok",
		Samples: []models.ProfileSample{
			{Label: "Ügyfélnek írt email", Content: "Szia! Köszönöm a megkeresést."},
		},
	}
	got := BuildProfileContext(profile)
	for _, want := range []string{
		"Fejlesztő és vállalkozó", "Backend rendszerek", "tegeződő",
		"Ügyfélnek írt email", "Köszönöm a megkeresést",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected context to contain %q, got: %s", want, got)
		}
	}
}

func TestBuildProfileContextSkipsSamplesWithEmptyContent(t *testing.T) {
	profile := &models.Profile{
		Samples: []models.ProfileSample{{Label: "Üres", Content: ""}},
	}
	if got := BuildProfileContext(profile); got != "" {
		t.Fatalf("expected empty string when only sample is blank, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestBuildProfileContext -v`
Expected: FAIL — `undefined: BuildProfileContext`

- [ ] **Step 3: Write the implementation**

`backend/internal/services/profile_context.go`:

```go
// backend/internal/services/profile_context.go
package services

import (
	"dev-bridge-manager/internal/models"
	"fmt"
	"strings"
)

// BuildProfileContext renders the profile into a single prompt-ready text
// block so every consumer (draft-reply today, future agents later) gets the
// same formatting instead of re-assembling it from the raw fields themselves.
func BuildProfileContext(profile *models.Profile) string {
	if profile == nil {
		return ""
	}

	var sections []string
	if strings.TrimSpace(profile.Background) != "" {
		sections = append(sections, "Háttér: "+profile.Background)
	}
	if strings.TrimSpace(profile.Expertise) != "" {
		sections = append(sections, "Szakterület: "+profile.Expertise)
	}
	if strings.TrimSpace(profile.ToneRules) != "" {
		sections = append(sections, "Kommunikációs stílus: "+profile.ToneRules)
	}

	var sampleLines []string
	for _, s := range profile.Samples {
		if strings.TrimSpace(s.Content) == "" {
			continue
		}
		label := s.Label
		if label == "" {
			label = "Minta"
		}
		sampleLines = append(sampleLines, fmt.Sprintf("- [%s]: %s", label, s.Content))
	}
	if len(sampleLines) > 0 {
		sections = append(sections, "Írásminták:\n"+strings.Join(sampleLines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestBuildProfileContext -v`
Expected: PASS (4/4)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/profile_context.go backend/internal/services/profile_context_test.go
git commit -m "feat(profile): add BuildProfileContext prompt-rendering service"
```

---

### Task 3: ProfileHandler (admin CRUD + internal context endpoint) + routes

**Files:**
- Create: `backend/internal/handlers/profile_handler.go`
- Create: `backend/internal/routes/profile_routes.go`
- Modify: `backend/internal/routes/routes.go`

**Interfaces:**
- Consumes: `models.Profile`, `models.ProfileSample`, `models.ProfileUpdateRequest`, `models.ProfileResponse` (Task 1); `services.BuildProfileContext` (Task 2); `database.GetDB()` (existing); `middleware.JWTMiddleware()` and `middleware.RequireRole(roleName string) fiber.Handler` (existing, `backend/internal/middleware/permission.go` and `auth.go` — unmodified).
- Produces: `handlers.NewProfileHandler() *ProfileHandler` with methods `GetProfile`, `UpdateProfile`, `GetProfileContext` (all `func(c *fiber.Ctx) error`), and `routes.SetupProfileRoutes(api fiber.Router)`. Registers `GET /api/v1/admin/profile`, `PUT /api/v1/admin/profile` (both `super_admin`-only), and `GET /api/v1/internal/profile-context` (no auth — internal docker-network call, same trust model as the AI service's own unauthenticated endpoints).

**Why role-gating happens per-route, not via `admin.Use(...)`:** `billingo_settings_routes.go` already registers its own `api.Group("/admin")` with `admin.Use(middleware.JWTMiddleware())`. Fiber matches middleware registered through `Group.Use()` against the request *path*, not the specific `Group` object that registered it — so calling `.Use(middleware.RequireRole("super_admin"))` on a second, separately-created `/admin` group here would silently apply to **every** `/admin/*` route in the app, including the existing Billingo settings screen, locking out plain `admin` users from a feature that has nothing to do with this one. `middleware.RequireRole("super_admin")` must instead be passed as a route-specific handler, e.g. `admin.Get("/profile", middleware.RequireRole("super_admin"), h.GetProfile)`, which only applies to that one route.

There is no DB-backed handler test in this codebase (`email_handler_test.go` only tests the pure `isValidEmailCategory` helper) and none of this handler's three methods are pure — `GetProfile`/`UpdateProfile`/`GetProfileContext` all touch the DB directly. Per the established convention, this task has no new Go test file; it is verified with real HTTP calls against the running container instead (Step 5).

- [ ] **Step 1: Write the handler**

`backend/internal/handlers/profile_handler.go`:

```go
// backend/internal/handlers/profile_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ProfileHandler struct{}

func NewProfileHandler() *ProfileHandler {
	return &ProfileHandler{}
}

func loadProfile(db *gorm.DB) (*models.Profile, error) {
	var profile models.Profile
	if err := db.Preload("Samples").First(&profile, 1).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetProfile - GET /api/v1/admin/profile (super_admin only, see routes/profile_routes.go)
func (h *ProfileHandler) GetProfile(c *fiber.Ctx) error {
	profile, err := loadProfile(database.GetDB())
	if err != nil {
		return c.Status(500).JSON(models.ProfileResponse{Success: false, Message: "Failed to load profile"})
	}
	return c.JSON(models.ProfileResponse{Success: true, Profile: profile})
}

// UpdateProfile - PUT /api/v1/admin/profile (super_admin only, see routes/profile_routes.go)
// Replaces the full samples list on every call - simpler than diffing, and
// the admin UI always submits the complete list back anyway.
func (h *ProfileHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var req models.ProfileUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.ProfileResponse{Success: false, Message: "Invalid request body"})
	}

	db := database.GetDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Profile{}).Where("id = ?", 1).Updates(map[string]interface{}{
			"background": req.Background,
			"expertise":  req.Expertise,
			"tone_rules": req.ToneRules,
			"updated_by": userID,
		}).Error; err != nil {
			return err
		}

		if err := tx.Where("profile_id = ?", 1).Delete(&models.ProfileSample{}).Error; err != nil {
			return err
		}

		for _, s := range req.Samples {
			if err := tx.Create(&models.ProfileSample{ProfileID: 1, Label: s.Label, Content: s.Content}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return c.Status(500).JSON(models.ProfileResponse{Success: false, Message: "Failed to update profile"})
	}

	profile, err := loadProfile(db)
	if err != nil {
		return c.Status(500).JSON(models.ProfileResponse{Success: false, Message: "Failed to load updated profile"})
	}
	return c.JSON(models.ProfileResponse{Success: true, Profile: profile})
}

// GetProfileContext - GET /api/v1/internal/profile-context - called by other
// services on the docker-internal network, not by end users; unauthenticated,
// mirroring the AI service's own unauthenticated endpoints (network
// isolation is the guard, same accepted risk model as ai:8000 today).
func (h *ProfileHandler) GetProfileContext(c *fiber.Ctx) error {
	profile, err := loadProfile(database.GetDB())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"context": ""})
	}
	return c.JSON(fiber.Map{"context": services.BuildProfileContext(profile)})
}
```

- [ ] **Step 2: Write the routes**

`backend/internal/routes/profile_routes.go`:

```go
// backend/internal/routes/profile_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupProfileRoutes(api fiber.Router) {
	h := handlers.NewProfileHandler()

	admin := api.Group("/admin")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only, deliberately no admin fallback - the profile holds
	// personal background/writing-style data the operator wants to keep to
	// themselves. Passed per-route (not via admin.Use()) so it doesn't leak
	// onto other /admin/* routes registered by other route files - see
	// profile_handler.go's doc comment for why that matters.
	admin.Get("/profile", middleware.RequireRole("super_admin"), h.GetProfile)
	admin.Put("/profile", middleware.RequireRole("super_admin"), h.UpdateProfile)

	internal := api.Group("/internal")
	internal.Get("/profile-context", h.GetProfileContext)
}
```

- [ ] **Step 3: Register the routes**

In `backend/internal/routes/routes.go`, add the call alongside the other `Setup*Routes(v1)` calls:

```go
	SetupEmailRoutes(v1)             // Synced email list/detail/attachment/send endpoints
	SetupProfileRoutes(v1)           // Global "who am I" profile - admin CRUD + internal context endpoint
```

- [ ] **Step 4: Build**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds with no errors.

- [ ] **Step 5: Manually verify the role gate against the running container**

Restart the backend to pick up the new routes:

Run: `docker restart devbridge_backend`

Get a JWT for a `super_admin` user and one for a plain `admin` user (via the existing login endpoint or however you normally obtain a dev token in this project), then:

```bash
# super_admin: expect 200
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/admin/profile -H "Authorization: Bearer <super_admin_token>"

# plain admin: expect 403
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/admin/profile -H "Authorization: Bearer <admin_token>"

# internal endpoint, no auth header: expect 200 with {"context":""}
curl -s http://localhost:8080/api/v1/internal/profile-context
```

Expected: `200`, `403`, and `{"context":""}` respectively. If the `admin` call returns `200`, stop — that means the role gate is misapplied (likely the `admin.Use()` mistake described above) and must be fixed before continuing.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/profile_handler.go backend/internal/routes/profile_routes.go backend/internal/routes/routes.go
git commit -m "feat(profile): add super_admin-only profile CRUD and internal context endpoint"
```

---

### Task 4: AI service `/draft-reply` endpoint

**Files:**
- Create: `ai/app/draft_reply.py`
- Modify: `ai/app/main.py`
- Create: `ai/tests/test_draft_reply.py`

**Interfaces:**
- Produces: `draft_reply_agent` (a pydantic-ai `Agent`), `DraftReplyRequest{email_content: str, profile_context: str = ""}`, `DraftReplyResult{draft: str}`, `async def draft_reply(req: DraftReplyRequest) -> str`. Registers `POST /draft-reply` on the FastAPI `app`. Consumed by Task 5's `DraftReplyService.DraftReply` (Go), which POSTs `{"email_content": ..., "profile_context": ...}` and reads back `{"draft": ...}`.

- [ ] **Step 1: Write the failing tests**

`ai/tests/test_draft_reply.py`:

```python
from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.draft_reply import draft_reply_agent
from app.main import app

client = TestClient(app)


def test_draft_reply_returns_generated_draft():
    with draft_reply_agent.override(model=TestModel(custom_output_args={"draft": "Szia! Köszönöm a megkeresésed."})):
        response = client.post(
            "/draft-reply",
            json={
                "email_content": "Szia, mikor tudnátok elkezdeni a projektet?",
                "profile_context": "Háttér: szoftverfejlesztő vállalkozó",
            },
        )
    assert response.status_code == 200
    assert response.json()["draft"] == "Szia! Köszönöm a megkeresésed."


def test_draft_reply_works_without_profile_context():
    with draft_reply_agent.override(model=TestModel(custom_output_args={"draft": "Köszönöm az emailt."})):
        response = client.post("/draft-reply", json={"email_content": "Csak egy teszt üzenet."})
    assert response.status_code == 200
    assert response.json()["draft"] == "Köszönöm az emailt."


def test_draft_reply_requires_email_content():
    response = client.post("/draft-reply", json={})
    assert response.status_code == 422
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `docker exec devbridge_ai pytest tests/test_draft_reply.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'app.draft_reply'`

- [ ] **Step 3: Write the implementation**

`ai/app/draft_reply.py`:

```python
import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Each agent module reads its own GEMINI_MODEL rather than importing it from
# categorize_email.py, to avoid a circular import - same reasoning as the
# comment in categorize_email.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class DraftReplyRequest(BaseModel):
    email_content: str
    profile_context: str = ""


class DraftReplyResult(BaseModel):
    draft: str


draft_reply_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=DraftReplyResult,
    defer_model_check=True,
    instructions=(
        "Draft a reply to a business email on behalf of the user. Write in "
        "Hungarian unless the original email is in another language. Keep "
        "the reply focused and only as long as the original email warrants. "
        "This is a draft suggestion only - a human will review and edit it "
        "before sending, so prefer a complete, ready-to-edit draft over "
        "placeholders."
    ),
)


async def draft_reply(req: DraftReplyRequest) -> str:
    prompt_parts = []
    if req.profile_context.strip():
        prompt_parts.append(
            "Az alábbi profil alapján fogalmazz választ a felhasználó nevében. "
            "Vedd figyelembe a hátterét, szakterületét és kommunikációs "
            "stílusát, az írásmintákat pedig hangnem-referenciaként "
            "használd.\n\n" + req.profile_context
        )
    prompt_parts.append(f"Beérkező email:\n{req.email_content}")

    result = await draft_reply_agent.run("\n\n".join(prompt_parts))
    return result.output.draft
```

In `ai/app/main.py`, add the import alongside the existing one:

```python
from app.categorize_email import CategorizeEmailRequest, CategorizeEmailResult, categorize_email
from app.draft_reply import DraftReplyRequest, DraftReplyResult, draft_reply
```

And add the endpoint after `categorize_email_endpoint`:

```python
@app.post("/draft-reply", response_model=DraftReplyResult)
async def draft_reply_endpoint(payload: DraftReplyRequest) -> DraftReplyResult:
    draft = await draft_reply(payload)
    return DraftReplyResult(draft=draft)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_ai pytest tests/test_draft_reply.py -v`
Expected: PASS (3/3)

Then run the full AI test suite to confirm no regression:

Run: `docker exec devbridge_ai pytest -q`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add ai/app/draft_reply.py ai/app/main.py ai/tests/test_draft_reply.py
git commit -m "feat(ai): add /draft-reply endpoint"
```

---

### Task 5: Backend DraftReplyService + EmailHandler.DraftReply endpoint

**Files:**
- Create: `backend/internal/services/draft_reply.go`
- Test: `backend/internal/services/draft_reply_test.go`
- Modify: `backend/internal/handlers/email_handler.go`
- Modify: `backend/internal/routes/email_routes.go`

**Interfaces:**
- Consumes: Task 4's AI service `/draft-reply` contract; `services.BuildProfileContext` (Task 2); existing `EmailHandler.gmailAPI services.GmailAPI` field, `currentGmailAccount(userID uint) (*models.GmailAccount, error)`, `database.GetDB()`, `models.Email` (all pre-existing, unmodified).
- Produces: `services.DraftReplier` interface with `DraftReply(ctx context.Context, emailContent, profileContext string) (string, error)`, implemented by `services.DraftReplyService` (constructed via `services.NewDraftReplyService()`). `EmailHandler` gains a `draftReplier services.DraftReplier` field and a `DraftReply(c *fiber.Ctx) error` method, registered as `POST /api/v1/emails/:id/draft-reply`.

- [ ] **Step 1: Write the failing tests for DraftReplyService**

`backend/internal/services/draft_reply_test.go`:

```go
// backend/internal/services/draft_reply_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDraftReplySendsFieldsAndReturnsDraft(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/draft-reply" {
			t.Errorf("expected path /draft-reply, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "Szia! Köszönöm a megkeresésed."})
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	got, err := svc.DraftReply(context.Background(), "email body", "profile context")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Szia! Köszönöm a megkeresésed." {
		t.Fatalf("expected draft text, got %q", got)
	}
	if gotBody["email_content"] != "email body" || gotBody["profile_context"] != "profile context" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestDraftReplyReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.DraftReply(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestDraftReplyReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "d"})
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.DraftReply(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestDraftReply -v`
Expected: FAIL — `undefined: DraftReplyService`

- [ ] **Step 3: Write DraftReplyService**

`backend/internal/services/draft_reply.go`:

```go
// backend/internal/services/draft_reply.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DraftReplier is the seam email_handler.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type DraftReplier interface {
	DraftReply(ctx context.Context, emailContent, profileContext string) (string, error)
}

var _ DraftReplier = (*DraftReplyService)(nil)

// DraftReplyService calls the AI service's /draft-reply endpoint. baseURL
// follows the same AI_SERVICE_URL convention as EmailCategorizationService.
// Timeout is longer than EmailCategorizationService's 10s: generating a full
// reply draft is a heavier generation task than a one-word categorization.
type DraftReplyService struct {
	httpClient *http.Client
	baseURL    string
}

func NewDraftReplyService() *DraftReplyService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &DraftReplyService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type draftReplyRequest struct {
	EmailContent   string `json:"email_content"`
	ProfileContext string `json:"profile_context"`
}

type draftReplyResponse struct {
	Draft string `json:"draft"`
}

func (s *DraftReplyService) DraftReply(ctx context.Context, emailContent, profileContext string) (string, error) {
	payload, err := json.Marshal(draftReplyRequest{EmailContent: emailContent, ProfileContext: profileContext})
	if err != nil {
		return "", fmt.Errorf("encoding draft-reply request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/draft-reply", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building draft-reply request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading draft-reply response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed draftReplyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing draft-reply response: %w", err)
	}

	return parsed.Draft, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestDraftReply -v`
Expected: PASS (3/3)

- [ ] **Step 5: Add EmailHandler.DraftReply**

In `backend/internal/handlers/email_handler.go`, change the struct and constructor:

```go
type EmailHandler struct {
	gmailAPI     services.GmailAPI
	draftReplier services.DraftReplier
}

func NewEmailHandler() *EmailHandler {
	return &EmailHandler{
		gmailAPI:     services.NewRealGmailAPI(),
		draftReplier: services.NewDraftReplyService(),
	}
}
```

Add `"strings"` to the existing import block, then add the new handler after `GetEmail` (mirrors its structure exactly: same account/id/email lookup, same error messages and status codes):

```go
// DraftReply - POST /api/v1/emails/:id/draft-reply - fetches the email body,
// combines it with the global profile context, and asks the AI service for
// a draft. Returns text only - never saves or sends anything.
func (h *EmailHandler) DraftReply(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Connect your Gmail account first"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid email id"})
	}

	var email models.Email
	if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", id, account.ID).First(&email).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Email not found"})
	}

	full, err := h.gmailAPI.GetFullMessage(c.Context(), account, email.GmailMessageID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to fetch email body from Gmail: " + err.Error()})
	}

	content := full.BodyText
	if strings.TrimSpace(content) == "" {
		content = full.Subject
	}
	const maxContentLen = 4000
	if len(content) > maxContentLen {
		content = content[:maxContentLen]
	}

	var profile models.Profile
	profileContext := ""
	if err := database.GetDB().Preload("Samples").First(&profile, 1).Error; err == nil {
		profileContext = services.BuildProfileContext(&profile)
	}
	// A missing/unreadable profile row degrades to no persona rather than
	// failing the whole request - drafting a plain reply is still useful.

	draft, err := h.draftReplier.DraftReply(c.Context(), content, profileContext)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to generate draft: " + err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "draft": draft})
}
```

- [ ] **Step 6: Register the route**

In `backend/internal/routes/email_routes.go`, add the new route inside the existing `emails` group (already behind `JWTMiddleware` + `RequirePermission("gmail.manage")`):

```go
	emails.Get("/:id", h.GetEmail)
	emails.Post("/:id/draft-reply", h.DraftReply)
	emails.Get("/:id/attachments/:attachmentId", h.GetAttachment)
```

- [ ] **Step 7: Build and run the full backend test suite**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds with no errors.

Run: `docker exec devbridge_backend go test ./...`
Expected: all tests pass, no regressions.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/draft_reply.go backend/internal/services/draft_reply_test.go backend/internal/handlers/email_handler.go backend/internal/routes/email_routes.go
git commit -m "feat(emails): add AI draft-reply endpoint using the global profile"
```

---

### Task 6: Frontend profile admin UI (super_admin only)

**Files:**
- Modify: `frontend/src/utils/permissions.ts`
- Create: `frontend/src/services/profileService.ts`
- Create: `frontend/src/components/ProfileModal.tsx`
- Modify: `frontend/src/components/dashboard/tabs/AdminTab.tsx`

**Interfaces:**
- Consumes: Task 3's `GET /api/v1/admin/profile` / `PUT /api/v1/admin/profile` (returning/accepting `{success, message?, profile?: {id, background, expertise, tone_rules, samples: [{id, profile_id, label, content}], updated_by, updated_at}}`); existing `apiClient.get`/`apiClient.put` (`frontend/src/lib/api.ts`, unmodified); existing `User` type (`frontend/src/types/user.ts`, has `role.name`).
- Produces: `isSuperAdmin(user: User | null): boolean` in `permissions.ts`. `ProfileService.getProfile()` / `ProfileService.updateProfile(data)` in `profileService.ts`, returning `ProfileData`. `<ProfileModal isOpen onClose />` component. An `AdminTab` card gated by `isSuperAdmin` (not `hasPermission`), matching the backend's strict role gate.

- [ ] **Step 1: Add isSuperAdmin helper**

In `frontend/src/utils/permissions.ts`, add after `isAdmin`:

```ts
export const isSuperAdmin = (user: User | null): boolean => {
    return user?.role?.name === 'super_admin'
}
```

- [ ] **Step 2: Write profileService.ts**

`frontend/src/services/profileService.ts`:

```ts
// frontend/src/services/profileService.ts
import { apiClient } from '@/lib/api'

export interface ProfileSample {
    id?: number
    label: string
    content: string
}

export interface ProfileData {
    background: string
    expertise: string
    tone_rules: string
    samples: ProfileSample[]
    updated_by: number
    updated_at: string
}

interface ProfileApiResponse {
    success: boolean
    message?: string
    profile?: {
        id: number
        background: string
        expertise: string
        tone_rules: string
        samples: ProfileSample[] | null
        updated_by: number
        updated_at: string
    }
}

export interface ProfileUpdateRequest {
    background: string
    expertise: string
    tone_rules: string
    samples: ProfileSample[]
}

export class ProfileService {
    private static baseUrl = '/admin/profile'

    static async getProfile(): Promise<ProfileData> {
        const response = await apiClient.get<ProfileApiResponse>(this.baseUrl)
        if (response.success && response.profile) {
            return {
                background: response.profile.background,
                expertise: response.profile.expertise,
                tone_rules: response.profile.tone_rules,
                samples: response.profile.samples || [],
                updated_by: response.profile.updated_by,
                updated_at: response.profile.updated_at,
            }
        }
        throw new Error(response.message || 'Failed to fetch profile')
    }

    static async updateProfile(data: ProfileUpdateRequest): Promise<ProfileData> {
        const response = await apiClient.put<ProfileApiResponse>(this.baseUrl, data)
        if (response.success && response.profile) {
            return {
                background: response.profile.background,
                expertise: response.profile.expertise,
                tone_rules: response.profile.tone_rules,
                samples: response.profile.samples || [],
                updated_by: response.profile.updated_by,
                updated_at: response.profile.updated_at,
            }
        }
        throw new Error(response.message || 'Failed to update profile')
    }
}
```

- [ ] **Step 3: Write ProfileModal.tsx**

`frontend/src/components/ProfileModal.tsx` (fetch-on-open + controlled-form pattern mirrors `BillingoSettingsModal.tsx`):

```tsx
// frontend/src/components/ProfileModal.tsx
import { useState, useEffect } from 'react'
import { ProfileService, ProfileSample } from '@/services/profileService'

interface ProfileModalProps {
    isOpen: boolean
    onClose: () => void
}

const SOFT_SAMPLE_WARNING_THRESHOLD = 8

export default function ProfileModal({ isOpen, onClose }: ProfileModalProps) {
    const [background, setBackground] = useState('')
    const [expertise, setExpertise] = useState('')
    const [toneRules, setToneRules] = useState('')
    const [samples, setSamples] = useState<ProfileSample[]>([])
    const [loading, setLoading] = useState(false)
    const [fetching, setFetching] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [success, setSuccess] = useState(false)

    useEffect(() => {
        if (isOpen) {
            setFetching(true)
            setSuccess(false)
            setError(null)
            ProfileService.getProfile()
                .then((profile) => {
                    setBackground(profile.background)
                    setExpertise(profile.expertise)
                    setToneRules(profile.tone_rules)
                    setSamples(profile.samples)
                })
                .catch((err) => setError(err.message))
                .finally(() => setFetching(false))
        }
    }, [isOpen])

    const handleClose = () => {
        if (!loading) {
            setError(null)
            setSuccess(false)
            onClose()
        }
    }

    const addSample = () => {
        setSamples(prev => [...prev, { label: '', content: '' }])
    }

    const updateSample = (index: number, field: 'label' | 'content', value: string) => {
        setSamples(prev => prev.map((s, i) => (i === index ? { ...s, [field]: value } : s)))
    }

    const removeSample = (index: number) => {
        setSamples(prev => prev.filter((_, i) => i !== index))
    }

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        try {
            setLoading(true)
            setError(null)
            const updated = await ProfileService.updateProfile({
                background,
                expertise,
                tone_rules: toneRules,
                samples: samples.filter(s => s.label.trim() || s.content.trim()),
            })
            setSamples(updated.samples)
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
            <div className="bg-card rounded-xl shadow-sm p-6 w-full max-w-2xl">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-semibold">AI Profil / Perszóna</h2>
                    <button
                        onClick={handleClose}
                        disabled={loading}
                        className="text-muted-foreground hover:text-foreground disabled:opacity-50"
                    >
                        ✕
                    </button>
                </div>

                {fetching ? (
                    <div className="text-sm text-muted-foreground py-4">Betöltés...</div>
                ) : (
                    <form onSubmit={handleSubmit} className="space-y-4">
                        {error && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {error}
                            </div>
                        )}
                        {success && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Profil elmentve
                            </div>
                        )}

                        <div>
                            <label htmlFor="background" className="block text-sm font-medium text-foreground mb-1">
                                Háttér
                            </label>
                            <textarea
                                id="background"
                                value={background}
                                onChange={(e) => setBackground(e.target.value)}
                                rows={3}
                                placeholder="Mivel foglalkozol, mi a szereped..."
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="expertise" className="block text-sm font-medium text-foreground mb-1">
                                Szakterület
                            </label>
                            <textarea
                                id="expertise"
                                value={expertise}
                                onChange={(e) => setExpertise(e.target.value)}
                                rows={3}
                                placeholder="Mihez értesz..."
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div>
                            <label htmlFor="tone_rules" className="block text-sm font-medium text-foreground mb-1">
                                Kommunikációs stílus
                            </label>
                            <textarea
                                id="tone_rules"
                                value={toneRules}
                                onChange={(e) => setToneRules(e.target.value)}
                                rows={3}
                                placeholder="Milyen hangnemben, stílusban kommunikálsz..."
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={loading}
                            />
                        </div>

                        <div className="space-y-2">
                            <div className="flex items-center justify-between">
                                <label className="block text-sm font-medium text-foreground">Írásminták</label>
                                <button
                                    type="button"
                                    onClick={addSample}
                                    disabled={loading}
                                    className="text-xs text-primary hover:underline disabled:opacity-50"
                                >
                                    + Új minta
                                </button>
                            </div>
                            {samples.length > SOFT_SAMPLE_WARNING_THRESHOLD && (
                                <p className="text-xs text-amber-600">
                                    {samples.length} minta van megadva - ennyi minta felett az AI nehezebben tudja kiemelni a jellemző stílust.
                                </p>
                            )}
                            {samples.map((sample, index) => (
                                <div key={index} className="border border-border rounded-lg p-3 space-y-2">
                                    <div className="flex items-center gap-2">
                                        <input
                                            type="text"
                                            value={sample.label}
                                            onChange={(e) => updateSample(index, 'label', e.target.value)}
                                            placeholder="Cím (pl. Ügyfélnek írt email)"
                                            className="flex-1 px-3 py-1.5 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                            disabled={loading}
                                        />
                                        <button
                                            type="button"
                                            onClick={() => removeSample(index)}
                                            disabled={loading}
                                            className="text-xs text-destructive hover:underline disabled:opacity-50"
                                        >
                                            Törlés
                                        </button>
                                    </div>
                                    <textarea
                                        value={sample.content}
                                        onChange={(e) => updateSample(index, 'content', e.target.value)}
                                        rows={3}
                                        placeholder="A minta szövege..."
                                        className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                        disabled={loading}
                                    />
                                </div>
                            ))}
                        </div>

                        <div className="flex space-x-3 pt-4">
                            <button
                                type="button"
                                onClick={handleClose}
                                disabled={loading}
                                className="flex-1 px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted disabled:opacity-50 transition-colors"
                            >
                                Bezárás
                            </button>
                            <button
                                type="submit"
                                disabled={loading}
                                className="flex-1 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary/90 disabled:opacity-50 transition-colors"
                            >
                                {loading ? 'Mentés...' : 'Mentés'}
                            </button>
                        </div>
                    </form>
                )}
            </div>
        </div>
    )
}
```

- [ ] **Step 4: Wire into AdminTab**

In `frontend/src/components/dashboard/tabs/AdminTab.tsx`, update the imports:

```tsx
import { hasPermission, isSuperAdmin } from '@/utils/permissions'
import BillingoSettingsModal from '@/components/BillingoSettingsModal'
import ProfileModal from '@/components/ProfileModal'
```

Add state:

```tsx
const [isBillingoModalOpen, setIsBillingoModalOpen] = useState(false)
const [isProfileModalOpen, setIsProfileModalOpen] = useState(false)
```

Add a card to `adminCards` (the array items need a `requireSuperAdmin` field, since this card must NOT be reachable through the permission system like the others - see filter change below):

```tsx
{
    title: "AI Profil / Perszóna",
    description: "Háttér, szakterület és írásminták beállítása, amit az AI-alapú funkciók a te stílusodban való válaszadáshoz használnak",
    buttonLabel: "Szerkesztés",
    permission: null,
    requireSuperAdmin: true,
    action: () => setIsProfileModalOpen(true),
    comingSoon: false
},
```

Change the filter to respect `requireSuperAdmin` (existing cards are untouched — they have no `requireSuperAdmin` field, so `card.requireSuperAdmin` is `undefined`/falsy and they fall through to the existing permission check unchanged):

```tsx
{adminCards
    .filter(card => (card as any).requireSuperAdmin ? isSuperAdmin(user) : (!card.permission || hasPermission(user, card.permission)))
    .map(card => (
```

Render the modal alongside the existing one:

```tsx
<BillingoSettingsModal
    isOpen={isBillingoModalOpen}
    onClose={() => setIsBillingoModalOpen(false)}
/>
<ProfileModal
    isOpen={isProfileModalOpen}
    onClose={() => setIsProfileModalOpen(false)}
/>
```

- [ ] **Step 5: Manually verify in the browser**

Run: `docker exec devbridge_frontend npm run build` (or confirm the dev server, already running under `npm run dev`, recompiles without errors)
Expected: no TypeScript/build errors.

Log in as a `super_admin` user, open Dashboard → Admin, confirm the "AI Profil / Perszóna" card is visible, open it, fill in a background/expertise/tone/sample, save, close and reopen to confirm it persisted. Log in as a plain `admin` user and confirm the card is **not** shown.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/utils/permissions.ts frontend/src/services/profileService.ts frontend/src/components/ProfileModal.tsx frontend/src/components/dashboard/tabs/AdminTab.tsx
git commit -m "feat(profile): add super_admin-only profile admin UI"
```

---

### Task 7: Frontend "AI válasz-javaslat" button on the Emails page

**Files:**
- Modify: `frontend/src/components/emails/ComposeEditor.tsx`
- Modify: `frontend/src/services/emailsService.ts`
- Modify: `frontend/src/app/dashboard/emails/page.tsx`

**Interfaces:**
- Consumes: Task 5's `POST /api/v1/emails/:id/draft-reply` (returns `{success, message?, draft?: string}`); existing `openCompose(reply?: EmailDetail)`, `composeEditorRef` (`useRef<ComposeEditorHandle>`), `selected: EmailDetail | null` state in `emails/page.tsx` (all pre-existing, unmodified).
- Produces: `ComposeEditorHandle.setText(text: string): void` (new method alongside existing `getHTML`/`getText`/`isEmpty`). `EmailsService.draftReply(id: number): Promise<{success: boolean; message?: string; draft?: string}>`. New `emails/page.tsx` state: `draftingReply: boolean`, `draftError: string | null`, and a "AI válasz-javaslat" button next to the existing "Válasz" button.

- [ ] **Step 1: Add setText to ComposeEditorHandle**

In `frontend/src/components/emails/ComposeEditor.tsx`, update the interface:

```ts
export interface ComposeEditorHandle {
    getHTML: () => string
    getText: () => string
    isEmpty: () => boolean
    setText: (text: string) => void
}
```

Add an escape helper above the `ComposeEditor` component definition (the AI draft is plain text that may contain characters that would otherwise be interpreted as HTML when passed to tiptap's `setContent`):

```ts
function escapeHtml(value: string): string {
    return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
```

Update `useImperativeHandle` to add `setText` (paragraphs on blank lines, single newlines become `<br>`, so multi-paragraph AI drafts keep their structure):

```ts
useImperativeHandle(ref, () => ({
    getHTML: () => editor?.getHTML() ?? '',
    getText: () => editor?.getText() ?? '',
    isEmpty: () => editor?.isEmpty ?? true,
    setText: (text: string) => {
        const html = text
            .split(/\n{2,}/)
            .map(paragraph => `<p>${escapeHtml(paragraph).replace(/\n/g, '<br>')}</p>`)
            .join('')
        editor?.commands.setContent(html)
    },
}), [editor])
```

- [ ] **Step 2: Add draftReply to EmailsService**

In `frontend/src/services/emailsService.ts`, add to the `EmailsService` object (after `send`):

```ts
    async draftReply(id: number): Promise<{ success: boolean; message?: string; draft?: string }> {
        // The AI service can take a while to generate a full draft, and the
        // backend's own AI-service HTTP client waits up to 30s - give the
        // browser a longer timeout so it doesn't give up before the backend
        // itself would (same reasoning as GmailService.sync's longer timeout).
        return apiClient.post(`/emails/${id}/draft-reply`, undefined, 40000)
    },
```

- [ ] **Step 3: Wire the button into emails/page.tsx**

Add new state near the other compose-related state (after `const [sendError, setSendError] = useState<string | null>(null)`):

```ts
const [draftingReply, setDraftingReply] = useState(false)
const [draftError, setDraftError] = useState<string | null>(null)
const [pendingDraftText, setPendingDraftText] = useState<string | null>(null)
```

Add a handler near `openCompose` (after it):

```ts
const handleDraftReply = async () => {
    if (!selected?.id) return
    setDraftError(null)
    openCompose(selected)
    try {
        setDraftingReply(true)
        const res = await EmailsService.draftReply(selected.id)
        if (!res.success || !res.draft) {
            setDraftError(res.message || 'Nem sikerült javaslatot generálni.')
            return
        }
        setPendingDraftText(res.draft)
    } catch (err: any) {
        setDraftError(err.message)
    } finally {
        setDraftingReply(false)
    }
}
```

Add an effect that fills the editor once it has mounted (opening the compose modal via `openCompose` happens synchronously in `handleDraftReply`, but `composeEditorRef.current` only exists after the modal's next render, so filling it must wait for that) — place it near the other effects, after the component's other `useEffect` calls:

```ts
useEffect(() => {
    if (composeOpen && pendingDraftText) {
        composeEditorRef.current?.setText(pendingDraftText)
        setPendingDraftText(null)
    }
}, [composeOpen, pendingDraftText])
```

Add the button next to the existing "Válasz" button (around line 491-496):

```tsx
<div className="flex items-center gap-2">
    <button
        onClick={() => openCompose(selected)}
        className="flex items-center gap-2 px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 transition-colors"
    >
        <RefreshCw size={14} /> Válasz
    </button>
    <button
        onClick={handleDraftReply}
        disabled={draftingReply}
        className="flex items-center gap-2 px-4 py-2 bg-primary/10 text-primary rounded-lg text-sm font-medium hover:bg-primary/20 disabled:opacity-50 transition-colors"
    >
        <Sparkles size={14} /> {draftingReply ? 'Javaslat készül...' : 'AI válasz-javaslat'}
    </button>
</div>
```

Add `Sparkles` to the existing `lucide-react` import at the top of the file (alongside `RefreshCw`, `Mail`, `Paperclip`, `X`, etc.).

Show `draftError` inside the compose modal, next to the existing `sendError` banner (in the `composeOpen &&` block, right after the `{sendError && (...)}` block):

```tsx
{draftError && (
    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
        {draftError}
    </div>
)}
```

- [ ] **Step 4: Manually verify in the browser**

Run: `docker exec devbridge_frontend npm run build` (or confirm the dev server recompiles without errors).

Open an email in the inbox, click "AI válasz-javaslat", confirm the compose modal opens immediately in a "Javaslat készül..." state, and once the AI service responds, the editor is pre-filled with the draft text (with paragraph breaks preserved) and editable before sending. Confirm clicking "Válasz" still works unchanged (empty editor, no AI call).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/emails/ComposeEditor.tsx frontend/src/services/emailsService.ts frontend/src/app/dashboard/emails/page.tsx
git commit -m "feat(emails): add AI válasz-javaslat button that pre-fills a drafted reply"
```

---

## Self-Review Notes

**Spec coverage:** All 7 spec components are covered — data model (Task 1), admin CRUD + super_admin gate (Task 3), internal context endpoint (Task 3), AI `/draft-reply` (Task 4), backend draft-reply integration (Task 5), admin UI (Task 6), frontend email-reply UI (Task 7). `BuildProfileContext` (Task 2) is the spec's "profile-context assembly" piece, factored into its own task since Task 3 and Task 5 both depend on it as a pure, independently-testable unit.

**Naming correction from spec:** the spec's illustrative model used `UpdatedByID uint`; Task 1 uses `UpdatedBy uint` instead, matching the exact field name already established by `models.BillingoSettings.UpdatedBy` in this codebase. Functionally identical, just matching existing convention precisely.

**Gating mechanism correction from spec:** the spec described "super_admin only, no admin fallback" as a policy requirement without specifying the exact code path. Task 3 uses the pre-existing `middleware.RequireRole("super_admin")` applied per-route (not via `admin.Use(...)`), and documents why a group-wide `.Use()` would silently over-broaden the gate to other admin screens sharing the `/admin` prefix. No new permission/role-check code was introduced.

**Type consistency check:** `models.ProfileResponse{Profile *models.Profile}` (Task 1) is what `ProfileHandler` (Task 3) returns and what `ProfileService.getProfile()`/`updateProfile()` (Task 6) parse — field names (`background`, `expertise`, `tone_rules`, `samples`, `updated_by`, `updated_at`) match exactly across the Go JSON tags and the TypeScript interfaces. `services.BuildProfileContext(profile *models.Profile) string` (Task 2) is called identically in Task 3's `GetProfileContext` and Task 5's `DraftReply`. `services.DraftReplier.DraftReply(ctx, emailContent, profileContext string) (string, error)` (Task 5) matches the field the `EmailHandler` struct declares and the method the AI service's `/draft-reply` JSON contract (Task 4) round-trips. `ComposeEditorHandle.setText` (Task 7) is added without touching the three existing methods any other caller relies on.

**Placeholder scan:** no TBD/TODO markers; every step includes complete, runnable code and exact shell commands.
