# AI E-mail Kategorizálás Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every new Gmail inbox message that lands during sync is automatically tagged with one of 5 fixed categories by the AI service, and the Emails page shows a colored badge per email plus a filter row to narrow the inbox list by category.

**Architecture:** The AI service (`ai/app/categorize_email.py`) gets a pydantic-ai `Agent` whose `output_type` is a Pydantic model with a `Literal[...]` category field, exposed via a new `POST /categorize-email` endpoint. The backend's existing `applyAddedMessages` sync loop (`gmail_sync.go`) calls a new `EmailCategorizationService` (plain HTTP client, same shape as `BillingoService`) for every inbox message before persisting it; on any error the email is still saved with `category = NULL` and the sync continues. `EmailListItem`/`ListEmails` expose and filter on the new column; the frontend adds a badge and a filter pill row.

**Tech Stack:** FastAPI + pydantic-ai + Gemini (AI service), Go/Fiber v2 + GORM + PostgreSQL (backend), Next.js/React/TypeScript + Tailwind (frontend).

**Spec:** `docs/superpowers/specs/2026-09-20-email-ai-categorization-design.md`

## Global Constraints

- Exactly 5 categories, fixed set, not user-extensible: `ugyfel`, `szamla`, `marketing`, `rendszeruzenet`, `egyeb`.
- Categorization runs only for `folder == "inbox"` messages, only at the point a new message is first inserted by `applyAddedMessages` (`backend/internal/services/gmail_sync.go:134`). No backfill/re-categorization of existing rows.
- An AI-call failure or an unrecognized category value must never abort the sync — the email is still saved, with `category = NULL`, and the failure is logged (same pattern as the existing `GetMessageMetadata` error handling at `gmail_sync.go:151`).
- New env var `AI_SERVICE_URL` (backend → AI service), default `http://ai:8000` when unset, matching the existing `os.Getenv` pattern (`auth_service.go`, `gmail_oauth.go`) — no new config package.
- No new infrastructure: no queue, no worker, no new dependency beyond what's already in `ai/pyproject.toml` (`pydantic-ai` is already present). The backend→AI call is a plain synchronous HTTP request, mirroring `billingo_service.go`.
- This repo has no DB-test convention (confirmed: no existing Go test touches `database.GetDB()`). Only pure, DB-independent functions get real Go `testing` unit tests. DB-touching wiring (`applyAddedMessages`, `ListEmails`) is verified via `go build`/`go vet` plus a manual smoke test against the running containers — not fabricated DB tests.
- All new/changed Go and Python code is verified inside the documented containers only: `docker exec devbridge_backend ...`, `docker exec devbridge_ai ...`, `docker exec devbridge_frontend ...` — never on the host.
- Migrations ship both `.up.sql` and `.down.sql`, matching migrations 000024–000028. Next number is `000029`.
- All new user-facing frontend copy is in Hungarian, matching the rest of the Emails page.
- The canonical list of valid category values lives once, in Go, as `models.ValidEmailCategories` (`backend/internal/models/email.go`) — both the categorization HTTP client and the `ListEmails` query-param validator reference it; no duplicated literal lists.

---

## File Structure

**AI service:**
- Create: `ai/app/categorize_email.py` — the pydantic-ai `Agent`, its request/result Pydantic models, and the `categorize_email()` function.
- Modify: `ai/app/main.py` — remove the now-dead `GEMINI_MODEL` constant (moves into `categorize_email.py`, its only consumer) and wire the new `POST /categorize-email` route.
- Create: `ai/tests/test_categorize_email.py` — pytest using `TestModel` (no real Gemini calls).

**Backend — new files:**
- `backend/migrations/000029_add_email_category.up.sql` / `.down.sql`
- `backend/internal/services/email_categorization.go` — `EmailCategorizer` interface + `EmailCategorizationService` (HTTP client).
- `backend/internal/services/email_categorization_test.go`

**Backend — modified files:**
- `backend/internal/models/email.go` — `Category *string` on `Email` and `EmailListItem`; new `ValidEmailCategories` map.
- `backend/internal/services/gmail_sync_helpers.go` — new pure helper `categorizeIfInbox`.
- `backend/internal/services/gmail_sync_helpers_test.go` — tests for `categorizeIfInbox`.
- `backend/internal/services/gmail_sync.go` — thread an `EmailCategorizer` parameter through `RunGmailSync` → `syncAccount` → `backfillAccount`/`applyAddedMessages`; use `categorizeIfInbox` before `db.Create`.
- `backend/internal/handlers/gmail_auth_handler.go` — pass a real `EmailCategorizationService` into `SyncAccountNow`.
- `backend/internal/handlers/email_handler.go` — `?category=` query param on `ListEmails`, new `isValidEmailCategory` helper.
- `backend/internal/handlers/email_handler_test.go` (new) — test for `isValidEmailCategory`.
- `.docker/docker-compose.yml` — `AI_SERVICE_URL: http://ai:8000` on the `backend` service.

**Frontend — modified files:**
- `frontend/src/services/emailsService.ts` — `category` field on `EmailListItem`; optional `category` param on `list()`.
- `frontend/src/app/dashboard/emails/page.tsx` — category badge on each row, filter pill row above the inbox list, category state wired into the existing fetch effect.

---

### Task 1: AI service — `/categorize-email` endpoint

**Files:**
- Create: `ai/app/categorize_email.py`
- Modify: `ai/app/main.py:1-8` (imports), `ai/app/main.py:46-48` (remove `GEMINI_MODEL`, add route)
- Test: `ai/tests/test_categorize_email.py`

**Interfaces:**
- Produces: `ai.app.categorize_email.CategorizeEmailRequest` (Pydantic model: `subject: str`, `snippet: str`, `from_address: str`, `from_name: str`), `ai.app.categorize_email.EmailCategory` (`Literal["ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"]`), `async def categorize_email(req: CategorizeEmailRequest) -> EmailCategory`. Task 3's Go `EmailCategorizationService` consumes the HTTP shape this produces (`{"category": "..."}` JSON response from `POST /categorize-email`), not the Python types directly.

- [ ] **Step 1: Write the failing test**

Create `ai/tests/test_categorize_email.py`:

```python
from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.categorize_email import categorize_agent
from app.main import app

client = TestClient(app)


def test_categorize_email_returns_one_of_the_five_categories():
    with categorize_agent.override(model=TestModel(custom_output_args={"category": "szamla"})):
        response = client.post(
            "/categorize-email",
            json={
                "subject": "Számla #123 esedékes",
                "snippet": "Kérjük egyenlítsd ki a mellékelt számlát.",
                "from_address": "billing@example.com",
                "from_name": "Példa Kft.",
            },
        )
    assert response.status_code == 200
    body = response.json()
    assert body["category"] in {"ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"}
    assert body["category"] == "szamla"


def test_categorize_email_requires_all_fields():
    response = client.post("/categorize-email", json={"subject": "Hi"})
    assert response.status_code == 422
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker exec devbridge_ai pytest tests/test_categorize_email.py -v`
Expected: FAIL with `ModuleNotFoundError: No module named 'app.categorize_email'` (or import error), since the module doesn't exist yet.

- [ ] **Step 3: Write the AI-side implementation**

Create `ai/app/categorize_email.py`:

```python
import os
from typing import Literal

from pydantic import BaseModel
from pydantic_ai import Agent

# Kept in sync manually with backend/internal/models/email.go's
# ValidEmailCategories - the two live in different languages, so there's
# no single source of truth to import from.
EmailCategory = Literal["ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"]

# Moved here from main.py: this Agent is the only consumer of the model
# name, and main.py importing it back would create a circular import.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class CategorizeEmailRequest(BaseModel):
    subject: str
    snippet: str
    from_address: str
    from_name: str


class CategorizeEmailResult(BaseModel):
    category: EmailCategory


categorize_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=CategorizeEmailResult,
    instructions=(
        "Categorize a business email into exactly one category based on its "
        "subject, snippet, and sender.\n"
        "- 'ugyfel': a client/customer writing about project work.\n"
        "- 'szamla': invoicing, billing, or payment related.\n"
        "- 'marketing': newsletters, promotions, advertising.\n"
        "- 'rendszeruzenet': an automated notification from a tool or "
        "service, not a person.\n"
        "- 'egyeb': anything that doesn't clearly fit the above."
    ),
)


async def categorize_email(req: CategorizeEmailRequest) -> EmailCategory:
    prompt = (
        f"From: {req.from_name} <{req.from_address}>\n"
        f"Subject: {req.subject}\n"
        f"Snippet: {req.snippet}"
    )
    result = await categorize_agent.run(prompt)
    return result.output.category
```

In `ai/app/main.py`, remove lines 46-48 (the `GEMINI_MODEL` constant and its comment — it moved into `categorize_email.py` above) and add the import plus the new route. The import goes with the other top-of-file imports; the route goes after the `/health` endpoint:

```python
# ai/app/main.py — add to the import block at the top:
from app.categorize_email import CategorizeEmailRequest, CategorizeEmailResult, categorize_email
```

```python
# ai/app/main.py — append after the /health endpoint:
@app.post("/categorize-email", response_model=CategorizeEmailResult)
async def categorize_email_endpoint(payload: CategorizeEmailRequest) -> CategorizeEmailResult:
    category = await categorize_email(payload)
    return CategorizeEmailResult(category=category)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker exec devbridge_ai pytest tests/test_categorize_email.py -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Run the full AI test suite to confirm no regression**

Run: `docker exec devbridge_ai pytest -v`
Expected: PASS (`test_health` + the 2 new tests)

- [ ] **Step 6: Commit**

```bash
git add ai/app/categorize_email.py ai/app/main.py ai/tests/test_categorize_email.py
git commit -m "feat(ai): add /categorize-email endpoint"
```

---

### Task 2: Backend — `category` column and model field

**Files:**
- Create: `backend/migrations/000029_add_email_category.up.sql`
- Create: `backend/migrations/000029_add_email_category.down.sql`
- Modify: `backend/internal/models/email.go`

**Interfaces:**
- Produces: `models.ValidEmailCategories map[string]bool` (5 keys: `ugyfel`, `szamla`, `marketing`, `rendszeruzenet`, `egyeb`), `Email.Category *string`, `EmailListItem.Category *string`. Tasks 3, 4, and 5 all consume `models.ValidEmailCategories`.

This task has no independent test of its own (a schema/struct-field change has nothing to unit-test in isolation); it's verified by `go build` and is exercised end-to-end by Task 4/5's tests and the final manual smoke test.

- [ ] **Step 1: Write the migration**

Create `backend/migrations/000029_add_email_category.up.sql`:

```sql
ALTER TABLE emails ADD COLUMN category VARCHAR(20);
```

Create `backend/migrations/000029_add_email_category.down.sql`:

```sql
ALTER TABLE emails DROP COLUMN category;
```

- [ ] **Step 2: Add the field and the shared category list to the model**

In `backend/internal/models/email.go`, add after the `EmailAttachmentMeta` struct (before `type Email struct`, around line 14):

```go
// EmailCategory* constants and ValidEmailCategories are the canonical list
// of categories the AI service (ai/app/categorize_email.py) can return -
// kept in sync manually since the two live in different languages.
const (
	EmailCategoryClient    = "ugyfel"
	EmailCategoryBilling   = "szamla"
	EmailCategoryMarketing = "marketing"
	EmailCategorySystem    = "rendszeruzenet"
	EmailCategoryOther     = "egyeb"
)

var ValidEmailCategories = map[string]bool{
	EmailCategoryClient:    true,
	EmailCategoryBilling:   true,
	EmailCategoryMarketing: true,
	EmailCategorySystem:    true,
	EmailCategoryOther:     true,
}
```

Add `Category *string` to `Email` (after `SyncedAt time.Time` at line 30):

```go
	SyncedAt       time.Time `json:"synced_at"`
	Category       *string   `json:"category" gorm:"size:20"`
```

Add `Category *string` to `EmailListItem` (after `ReceivedAt time.Time` at line 66):

```go
	ReceivedAt     time.Time             `json:"received_at"`
	Category       *string               `json:"category"`
```

- [ ] **Step 3: Apply the migration**

Run: `docker compose -f .docker/docker-compose.yml restart backend`

Verify: `docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "\d emails"` shows the new `category` column (adjust user/db name from `.docker/docker-compose.yml` if this fails — see `DATABASE_URL` in the `backend` service block).

- [ ] **Step 4: Confirm the build still compiles**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/000029_add_email_category.up.sql backend/migrations/000029_add_email_category.down.sql backend/internal/models/email.go
git commit -m "feat(emails): add category column and ValidEmailCategories"
```

---

### Task 3: Backend — `EmailCategorizationService` HTTP client

**Files:**
- Create: `backend/internal/services/email_categorization.go`
- Test: `backend/internal/services/email_categorization_test.go`
- Modify: `.docker/docker-compose.yml:8-15` (backend service `environment` block)

**Interfaces:**
- Consumes: `models.ValidEmailCategories` (Task 2).
- Produces: `type EmailCategorizer interface { Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error) }`, `func NewEmailCategorizationService() *EmailCategorizationService` (implements `EmailCategorizer`). Task 4 consumes both the interface type and the constructor.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/services/email_categorization_test.go`:

```go
// backend/internal/services/email_categorization_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCategorizeSendsFieldsAndReturnsCategory(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/categorize-email" {
			t.Errorf("expected path /categorize-email, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"category": "szamla"})
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	got, err := svc.Categorize(context.Background(), "Számla #1", "snippet", "billing@x.com", "Billing Co")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != "szamla" {
		t.Fatalf("expected category 'szamla', got %v", got)
	}
	if gotBody["subject"] != "Számla #1" || gotBody["from_address"] != "billing@x.com" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestCategorizeReturnsErrorOnUnknownCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"category": "not-a-real-category"})
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.Categorize(context.Background(), "s", "s", "a", "n")
	if err == nil {
		t.Fatal("expected an error for an unrecognized category, got nil")
	}
}

func TestCategorizeReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.Categorize(context.Background(), "s", "s", "a", "n")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestCategorizeReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"category": "egyeb"})
	}))
	defer server.Close()

	svc := &EmailCategorizationService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.Categorize(context.Background(), "s", "s", "a", "n")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestCategorize -v`
Expected: FAIL to compile — `EmailCategorizationService`/`baseURL`/`httpClient`/`Categorize` don't exist yet.

- [ ] **Step 3: Write the implementation**

Create `backend/internal/services/email_categorization.go`:

```go
// backend/internal/services/email_categorization.go
package services

import (
	"bytes"
	"context"
	"dev-bridge-manager/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// EmailCategorizer is the seam gmail_sync.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type EmailCategorizer interface {
	Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error)
}

// EmailCategorizationService calls the AI service's /categorize-email
// endpoint. baseURL defaults to the AI service's docker-network address but
// is overridable via AI_SERVICE_URL for other environments, and is set
// directly (bypassing the env var) in tests to point at an httptest.Server.
type EmailCategorizationService struct {
	httpClient *http.Client
	baseURL    string
}

func NewEmailCategorizationService() *EmailCategorizationService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &EmailCategorizationService{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
	}
}

type categorizeEmailRequest struct {
	Subject     string `json:"subject"`
	Snippet     string `json:"snippet"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

type categorizeEmailResponse struct {
	Category string `json:"category"`
}

func (s *EmailCategorizationService) Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error) {
	payload, err := json.Marshal(categorizeEmailRequest{
		Subject:     subject,
		Snippet:     snippet,
		FromAddress: fromAddress,
		FromName:    fromName,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding categorize-email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/categorize-email", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building categorize-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading categorize-email response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed categorizeEmailResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing categorize-email response: %w", err)
	}

	if !models.ValidEmailCategories[parsed.Category] {
		return nil, fmt.Errorf("ai service returned unknown category %q", parsed.Category)
	}

	return &parsed.Category, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestCategorize -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Add the env var to docker-compose**

In `.docker/docker-compose.yml`, in the `backend` service's `environment` block (after `GOOGLE_OAUTH_REDIRECT_URL`, around line 15):

```yaml
      GOOGLE_OAUTH_REDIRECT_URL: ${GOOGLE_OAUTH_REDIRECT_URL}
      AI_SERVICE_URL: http://ai:8000
```

- [ ] **Step 6: Apply and verify**

Run: `docker compose -f .docker/docker-compose.yml up -d --build backend`
Verify: `docker exec devbridge_backend env | grep AI_SERVICE_URL` prints `AI_SERVICE_URL=http://ai:8000`.

- [ ] **Step 7: Run the full backend test suite to confirm no regression**

Run: `docker exec devbridge_backend go test ./... `
Expected: PASS, no new failures.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/email_categorization.go backend/internal/services/email_categorization_test.go .docker/docker-compose.yml
git commit -m "feat(emails): add EmailCategorizationService AI client"
```

---

### Task 4: Backend — wire categorization into the Gmail sync path

**Files:**
- Modify: `backend/internal/services/gmail_sync_helpers.go`
- Modify: `backend/internal/services/gmail_sync_helpers_test.go`
- Modify: `backend/internal/services/gmail_sync.go:23-179`
- Modify: `backend/internal/handlers/gmail_auth_handler.go:84`

**Interfaces:**
- Consumes: `services.EmailCategorizer`, `services.NewEmailCategorizationService()` (Task 3); `models.Email.Category`, `models.EmailListItem.Category` (Task 2).
- Produces: `func categorizeIfInbox(ctx context.Context, categorizer EmailCategorizer, meta *GmailMessageMeta) *string` — a pure orchestration helper with no DB access, so it's unit-testable per this repo's testing convention (see Global Constraints). No other task consumes this directly; it's internal wiring for this task only.

- [ ] **Step 1: Write the failing test for the new pure helper**

Add to `backend/internal/services/gmail_sync_helpers_test.go` (new imports `context` and `errors` — `errors` is already imported):

```go
type fakeCategorizer struct {
	called  bool
	gotArgs [4]string // subject, snippet, fromAddress, fromName
	result  *string
	err     error
}

func (f *fakeCategorizer) Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error) {
	f.called = true
	f.gotArgs = [4]string{subject, snippet, fromAddress, fromName}
	return f.result, f.err
}

func TestCategorizeIfInboxSkipsSentFolder(t *testing.T) {
	cat := &fakeCategorizer{}
	meta := &GmailMessageMeta{Folder: "sent", Subject: "hi"}

	got := categorizeIfInbox(context.Background(), cat, meta)

	if got != nil {
		t.Errorf("expected nil category for sent folder, got %v", got)
	}
	if cat.called {
		t.Error("expected categorizer not to be called for sent folder")
	}
}

func TestCategorizeIfInboxCallsCategorizerForInbox(t *testing.T) {
	category := "ugyfel"
	cat := &fakeCategorizer{result: &category}
	meta := &GmailMessageMeta{Folder: "inbox", Subject: "hi", Snippet: "snip", FromAddress: "a@b.com", FromName: "A"}

	got := categorizeIfInbox(context.Background(), cat, meta)

	if got == nil || *got != "ugyfel" {
		t.Fatalf("expected category 'ugyfel', got %v", got)
	}
	if cat.gotArgs != [4]string{"hi", "snip", "a@b.com", "A"} {
		t.Errorf("categorizer called with unexpected args: %+v", cat.gotArgs)
	}
}

func TestCategorizeIfInboxReturnsNilOnCategorizerError(t *testing.T) {
	cat := &fakeCategorizer{err: errors.New("ai service down")}
	meta := &GmailMessageMeta{Folder: "inbox", Subject: "hi"}

	got := categorizeIfInbox(context.Background(), cat, meta)

	if got != nil {
		t.Errorf("expected nil category on categorizer error, got %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestCategorizeIfInbox -v`
Expected: FAIL to compile — `categorizeIfInbox` doesn't exist yet.

- [ ] **Step 3: Implement `categorizeIfInbox`**

In `backend/internal/services/gmail_sync_helpers.go`, add `"context"` and `"log"` to the import block (alongside `"encoding/base64"`, `"errors"`, `"strings"`), then add this function (e.g. after `classifyFolder`):

```go
// categorizeIfInbox asks the categorizer for meta's category when meta is
// an inbox message. On any categorizer error, or for non-inbox messages,
// it returns nil rather than failing — the sync must not stop just because
// the AI service is unreachable or returned something unexpected (see
// EmailCategorizationService.Categorize's own validation).
func categorizeIfInbox(ctx context.Context, categorizer EmailCategorizer, meta *GmailMessageMeta) *string {
	if meta.Folder != "inbox" {
		return nil
	}
	category, err := categorizer.Categorize(ctx, meta.Subject, meta.Snippet, meta.FromAddress, meta.FromName)
	if err != nil {
		log.Printf("gmail sync: failed to categorize message %s: %v", meta.GmailMessageID, err)
		return nil
	}
	return category
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestCategorizeIfInbox -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Thread the categorizer through the sync call chain**

In `backend/internal/services/gmail_sync.go`, update these four signatures and their bodies:

Replace lines 33-44 (`RunGmailSync`):

```go
func RunGmailSync(api GmailAPI, categorizer EmailCategorizer) {
	var accounts []models.GmailAccount
	if err := database.GetDB().Find(&accounts).Error; err != nil {
		log.Printf("gmail sync: failed to load accounts: %v", err)
		return
	}
	for i := range accounts {
		if err := syncAccount(context.Background(), api, &accounts[i], categorizer); err != nil {
			log.Printf("gmail sync: account %d failed: %v", accounts[i].ID, err)
		}
	}
}
```

Replace lines 23-31 (`StartGmailSyncScheduler`) to construct and pass a real categorizer:

```go
func StartGmailSyncScheduler() {
	api := NewRealGmailAPI()
	categorizer := NewEmailCategorizationService()
	RunGmailSync(api, categorizer)

	ticker := time.NewTicker(gmailSyncInterval)
	for range ticker.C {
		RunGmailSync(api, categorizer)
	}
}
```

Replace lines 49-51 (`SyncAccountNow`):

```go
func SyncAccountNow(ctx context.Context, api GmailAPI, account *models.GmailAccount, categorizer EmailCategorizer) error {
	return syncAccount(ctx, api, account, categorizer)
}
```

Replace lines 53-93 (`syncAccount`) — only the signature and the two calls into `backfillAccount`/`applyAddedMessages` change, the rest of the body is unchanged:

```go
func syncAccount(ctx context.Context, api GmailAPI, account *models.GmailAccount, categorizer EmailCategorizer) error {
	db := database.GetDB()

	if account.LastHistoryID == "" {
		cutoff := time.Now().AddDate(0, 0, -gmailInitialBackfillDays)
		if err := backfillAccount(ctx, api, db, account, cutoff, categorizer); err != nil {
			return recordSyncFailure(db, account, err)
		}
	} else {
		added, deleted, newHistoryID, err := api.ListHistory(ctx, account, account.LastHistoryID)
		if err != nil {
			if isHistoryExpiredError(err) {
				cutoff := time.Now()
				if account.LastSyncedAt != nil {
					cutoff = *account.LastSyncedAt
				}
				if err := backfillAccount(ctx, api, db, account, cutoff, categorizer); err != nil {
					return recordSyncFailure(db, account, err)
				}
			} else {
				return recordSyncFailure(db, account, err)
			}
		} else {
			if err := applyAddedMessages(ctx, api, db, account, added, categorizer); err != nil {
				return recordSyncFailure(db, account, err)
			}
			if len(deleted) > 0 {
				db.Where("gmail_account_id = ? AND gmail_message_id IN ?", account.ID, deleted).Delete(&models.Email{})
			}
			db.Model(account).Updates(map[string]interface{}{"last_history_id": newHistoryID, "needs_reauth": false})
			account.LastHistoryID = newHistoryID
		}
	}

	now := time.Now()
	db.Model(account).Update("last_synced_at", now)
	account.LastSyncedAt = &now

	purgeOldEmails(db, account.ID)
	return nil
}
```

Replace lines 114-132 (`backfillAccount`):

```go
func backfillAccount(ctx context.Context, api GmailAPI, db *gorm.DB, account *models.GmailAccount, after time.Time, categorizer EmailCategorizer) error {
	query := fmt.Sprintf("after:%d", after.Unix())
	ids, err := api.ListMessageIDs(ctx, account, query)
	if err != nil {
		return err
	}

	if err := applyAddedMessages(ctx, api, db, account, ids, categorizer); err != nil {
		return err
	}

	historyID, err := api.GetProfileHistoryID(ctx, account)
	if err != nil {
		return err
	}
	db.Model(account).Updates(map[string]interface{}{"last_history_id": historyID, "needs_reauth": false})
	account.LastHistoryID = historyID
	return nil
}
```

Replace lines 134-179 (`applyAddedMessages`) — adds the `categorizer` parameter and calls `categorizeIfInbox` before `db.Create`:

```go
func applyAddedMessages(ctx context.Context, api GmailAPI, db *gorm.DB, account *models.GmailAccount, ids []string, categorizer EmailCategorizer) error {
	if len(ids) == 0 {
		return nil
	}

	var existing []string
	db.Model(&models.Email{}).
		Where("gmail_account_id = ? AND gmail_message_id IN ?", account.ID, ids).
		Pluck("gmail_message_id", &existing)
	existingSet := make(map[string]bool, len(existing))
	for _, id := range existing {
		existingSet[id] = true
	}

	for _, id := range pendingMessageIDs(ids, existingSet) {
		meta, err := api.GetMessageMetadata(ctx, account, id)
		if err != nil {
			log.Printf("gmail sync: failed to fetch message %s: %v", id, err)
			continue
		}
		if meta == nil {
			continue // skipped: no INBOX/SENT label (see classifyFolder)
		}

		category := categorizeIfInbox(ctx, categorizer, meta)

		if err := db.Create(&models.Email{
			GmailAccountID: account.ID,
			GmailMessageID: meta.GmailMessageID,
			ThreadID:       meta.ThreadID,
			Folder:         meta.Folder,
			FromAddress:    meta.FromAddress,
			FromName:       meta.FromName,
			ToAddresses:    meta.ToAddresses,
			Subject:        meta.Subject,
			Snippet:        meta.Snippet,
			HasAttachments: len(meta.Attachments) > 0,
			AttachmentMeta: models.AttachmentMetaToJSON(meta.Attachments),
			IsRead:         meta.IsRead,
			ReceivedAt:     meta.ReceivedAt,
			SyncedAt:       time.Now(),
			Category:       category,
		}).Error; err != nil {
			log.Printf("gmail sync: failed to store message %s for account %d: %v", meta.GmailMessageID, account.ID, err)
			continue
		}
	}
	return nil
}
```

- [ ] **Step 6: Update the one other call site**

In `backend/internal/handlers/gmail_auth_handler.go:84`, change:

```go
	if err := services.SyncAccountNow(c.Context(), services.NewRealGmailAPI(), &account); err != nil {
```

to:

```go
	if err := services.SyncAccountNow(c.Context(), services.NewRealGmailAPI(), &account, services.NewEmailCategorizationService()); err != nil {
```

- [ ] **Step 7: Build and run the full backend test suite**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors.

Run: `docker exec devbridge_backend go vet ./...`
Expected: no warnings.

Run: `docker exec devbridge_backend go test ./...`
Expected: PASS, no new failures.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/gmail_sync_helpers.go backend/internal/services/gmail_sync_helpers_test.go backend/internal/services/gmail_sync.go backend/internal/handlers/gmail_auth_handler.go
git commit -m "feat(emails): categorize new inbox messages during Gmail sync"
```

---

### Task 5: Backend — filter `ListEmails` by category

**Files:**
- Modify: `backend/internal/handlers/email_handler.go:36-80`
- Create: `backend/internal/handlers/email_handler_test.go`

**Interfaces:**
- Consumes: `models.ValidEmailCategories` (Task 2), `models.Email.Category`/`EmailListItem.Category` (Task 2).
- Produces: `func isValidEmailCategory(category string) bool` — internal to this task, but follows the same naming convention as `handlers.isValidEmailType` (`email_template_handler.go:24`) for consistency.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/handlers/email_handler_test.go`:

```go
// backend/internal/handlers/email_handler_test.go
package handlers

import "testing"

func TestIsValidEmailCategory(t *testing.T) {
	valid := []string{"ugyfel", "szamla", "marketing", "rendszeruzenet", "egyeb"}
	for _, c := range valid {
		if !isValidEmailCategory(c) {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []string{"", "bogus", "UGYFEL"}
	for _, c := range invalid {
		if isValidEmailCategory(c) {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/handlers/... -run TestIsValidEmailCategory -v`
Expected: FAIL to compile — `isValidEmailCategory` doesn't exist yet.

- [ ] **Step 3: Implement the helper and wire it into `ListEmails`**

In `backend/internal/handlers/email_handler.go`, add after `currentGmailAccount` (around line 34):

```go
func isValidEmailCategory(category string) bool {
	return models.ValidEmailCategories[category]
}
```

Then in `ListEmails` (lines 37-80), add the query param handling right after the existing `folder` validation (after line 47) and add the filter to the query (after line 56):

```go
	category := c.Query("category")
	if category != "" && !isValidEmailCategory(category) {
		return c.Status(400).JSON(models.EmailListResponse{Success: false, Message: "invalid category"})
	}
```

```go
	db := database.GetDB().Model(&models.Email{}).Where("gmail_account_id = ? AND folder = ?", account.ID, folder)
	if category != "" {
		db = db.Where("category = ?", category)
	}
	db.Count(&total)
```

And add `Category: e.Category` to the `EmailListItem` construction in the loop (after `ReceivedAt: e.ReceivedAt,` around line 75):

```go
			ReceivedAt:     e.ReceivedAt,
			Category:       e.Category,
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/handlers/... -run TestIsValidEmailCategory -v`
Expected: PASS

- [ ] **Step 5: Build and run the full backend test suite**

Run: `docker exec devbridge_backend go build ./...`
Run: `docker exec devbridge_backend go test ./...`
Expected: both clean, no new failures.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/email_handler.go backend/internal/handlers/email_handler_test.go
git commit -m "feat(emails): filter ListEmails by category"
```

---

### Task 6: Frontend — `category` in `emailsService.ts`

**Files:**
- Modify: `frontend/src/services/emailsService.ts`

**Interfaces:**
- Produces: `EmailListItem.category: string | null`, `EmailsService.list(folder, page?, category?)`. Task 7 consumes both.

No test file — this repo has no frontend unit tests for services (confirmed: `emailsService.ts` has no existing `*.test.ts` sibling); verified via `tsc --noEmit` and the manual smoke test in Task 7.

- [ ] **Step 1: Add the field and param**

In `frontend/src/services/emailsService.ts`, add `category` to `EmailListItem` (after `is_read: boolean` at line 20):

```ts
    is_read: boolean
    category: string | null
    received_at: string
```

Update `list()` (lines 55-57):

```ts
    async list(folder: 'inbox' | 'sent', page = 1, category?: string): Promise<EmailListResponse> {
        const params: Record<string, string | number> = { folder, page }
        if (category) params.category = category
        return apiClient.get<EmailListResponse>('/emails', params)
    },
```

- [ ] **Step 2: Verify the type-check passes**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no errors (Task 7 hasn't changed the caller yet, so the extra optional param is backward-compatible).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/services/emailsService.ts
git commit -m "feat(emails): add category to EmailsService"
```

---

### Task 7: Frontend — category badge and filter on the Emails page

**Files:**
- Modify: `frontend/src/app/dashboard/emails/page.tsx`

**Interfaces:**
- Consumes: `EmailsService.list(folder, page, category)`, `EmailListItem.category` (Task 6).

No automated test — this repo has no frontend component test setup (confirmed: no `*.test.tsx` files under `frontend/src/app`). Verified via `tsc --noEmit` and a manual browser smoke test.

- [ ] **Step 1: Add the category style map and state**

In `frontend/src/app/dashboard/emails/page.tsx`, add after `AVATAR_COLORS`/`avatarColor` (after line 26):

```ts
const CATEGORY_STYLES: Record<string, { label: string; className: string }> = {
    ugyfel: { label: 'Ügyfél', className: 'bg-blue-500/10 text-blue-600' },
    szamla: { label: 'Számla', className: 'bg-emerald-500/10 text-emerald-600' },
    marketing: { label: 'Marketing', className: 'bg-amber-500/10 text-amber-600' },
    rendszeruzenet: { label: 'Rendszer', className: 'bg-slate-500/10 text-slate-600' },
    egyeb: { label: 'Egyéb', className: 'bg-muted text-muted-foreground' },
}
```

Add category filter state after `const [folder, setFolder] = useState<Folder>('inbox')` (line 83):

```ts
    const [category, setCategory] = useState<string | undefined>(undefined)
```

- [ ] **Step 2: Wire category into the fetch functions and effect**

Update `refreshEmails` (lines 113-124):

```ts
    const refreshEmails = () => {
        setLoadingEmails(true)
        setListError(null)
        setEmailsPage(1)
        return EmailsService.list(folder, 1, category)
            .then(res => {
                setEmails(res.emails || [])
                setEmailsTotal(res.total || 0)
            })
            .catch((err: any) => setListError(err.message))
            .finally(() => setLoadingEmails(false))
    }
```

Update `loadMoreEmails` (lines 126-138):

```ts
    const loadMoreEmails = () => {
        const nextPage = emailsPage + 1
        setLoadingMore(true)
        setListError(null)
        EmailsService.list(folder, nextPage, category)
            .then(res => {
                setEmails(prev => [...prev, ...(res.emails || [])])
                setEmailsTotal(res.total || 0)
                setEmailsPage(nextPage)
            })
            .catch((err: any) => setListError(err.message))
            .finally(() => setLoadingMore(false))
    }
```

Update the effect that triggers `refreshEmails` (lines 140-143):

```ts
    useEffect(() => {
        if (!status?.connected) return
        refreshEmails()
    }, [status?.connected, folder, category])
```

Reset `category` when switching away from `inbox` (since `sent` has no categories) — update the folder-tab `onClick` (lines 343-345):

```ts
                            <button
                                key={f}
                                onClick={() => { setFolder(f); setCategory(undefined); setSelected(null); setSelectedId(null) }}
```

- [ ] **Step 3: Add the filter pill row above the inbox list**

Insert right after the folder-tab `<div className="flex border-b border-border">...</div>` block closes (after line 355, before `{loadingEmails ? (` on line 357):

```tsx
                    {folder === 'inbox' && (
                        <div className="flex flex-wrap gap-1.5 px-4 py-2 border-b border-border">
                            <button
                                onClick={() => setCategory(undefined)}
                                className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                                    !category ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground hover:bg-muted/70'
                                }`}
                            >
                                Mind
                            </button>
                            {Object.entries(CATEGORY_STYLES).map(([key, { label }]) => (
                                <button
                                    key={key}
                                    onClick={() => setCategory(key)}
                                    className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                                        category === key ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground hover:bg-muted/70'
                                    }`}
                                >
                                    {label}
                                </button>
                            ))}
                        </div>
                    )}
```

- [ ] **Step 4: Render the badge on each email row**

In the row rendering block, update the subject line (lines 392-395) to include the badge:

```tsx
                                            <div className={`text-sm truncate flex items-center gap-1.5 ${!item.is_read ? 'font-medium text-foreground' : 'text-muted-foreground'}`}>
                                                <span className="truncate">{item.subject || '(nincs tárgy)'}</span>
                                                {item.has_attachments && <Paperclip size={12} className="inline shrink-0" />}
                                                {item.category && CATEGORY_STYLES[item.category] && (
                                                    <span className={`shrink-0 px-1.5 py-0.5 rounded-full text-[10px] font-semibold ${CATEGORY_STYLES[item.category].className}`}>
                                                        {CATEGORY_STYLES[item.category].label}
                                                    </span>
                                                )}
                                            </div>
```

(This replaces the previous single-line subject `<div>` that had the paperclip icon inline with `align-text-top` — the `flex items-center gap-1.5` wrapper keeps subject, attachment icon, and badge aligned on one row without needing that CSS trick anymore.)

- [ ] **Step 5: Verify the type-check passes**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Manual smoke test**

With the stack running (`docker compose -f .docker/docker-compose.yml up -d`):
1. Open the Emails page, confirm the inbox list still loads and looks correct (no layout regression from the subject-line change).
2. Trigger a sync (the refresh icon) so at least one new inbox message gets categorized; confirm a colored badge appears next to its subject.
3. Click through each filter pill; confirm the list narrows to only that category, and "Mind" resets it.
4. Switch to the "Elküldött" (sent) tab; confirm the filter pill row disappears and no badges are expected there (sent mail has no category).
5. Check the backend logs (`docker logs devbridge_backend --tail 50`) during the sync in step 2 — if the AI service was reachable there should be no `failed to categorize message` lines; if you want to also confirm the failure path, temporarily stop the `ai` container (`docker stop devbridge_ai`), trigger another sync, confirm new inbox messages still appear (with no badge) and the log shows `failed to categorize message ...: calling ai service: ...`, then restart it (`docker start devbridge_ai`).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app/dashboard/emails/page.tsx
git commit -m "feat(emails): add category badge and filter to Emails page"
```

---

## Self-Review Notes

- **Spec coverage:** 5 fixed categories (Task 1, 2), inbox-only + new-messages-only (Task 4's `categorizeIfInbox` + no backfill task), AI-failure → `NULL` + sync continues (Task 4), badge + filterable UI (Task 7), `AI_SERVICE_URL` env var (Task 3), migration (Task 2), backend/AI tests (Tasks 1, 3, 4, 5) — all spec sections have a covering task.
- **Type consistency checked:** `EmailCategorizer.Categorize` signature is identical across its interface declaration (Task 3), the `fakeCategorizer` test double (Task 4), and every call site (Task 4). `models.ValidEmailCategories` is defined once (Task 2) and referenced (not redefined) in Tasks 3 and 5. `EmailListItem.category` (frontend, Task 6) matches the JSON key `category` emitted by `EmailListItem.Category *string` (backend, Task 2).
- **No placeholders:** every step has concrete code, not a description of code.
