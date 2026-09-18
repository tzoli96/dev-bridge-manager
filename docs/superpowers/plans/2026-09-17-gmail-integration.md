# Gmail Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the (single, super_admin) user connect their Gmail account, browse a local mirror of every received/sent email under a new "Emails" nav item, compose/reply to emails, and send an invoice-notice email to a client directly from a project's invoice page before issuing the real invoice.

**Architecture:** OAuth2 tokens for one Gmail account are stored in a new `gmail_accounts` table. A ticker-based background job (mirroring `services.StartAutoInvoiceScheduler`) runs every 3 hours, walking the Gmail History API since the last sync (or doing a 30-day `after:` backfill on first connect) and upserting message metadata (no bodies/attachments) into a new `emails` table. The Emails page lists/reads from this local mirror and lazily fetches body/attachment bytes live from Gmail on open/download. Sending (compose, reply, invoice notice) always calls the Gmail API directly — nothing is queued.

**Tech Stack:** Go/Fiber v2 + GORM + PostgreSQL (backend), Next.js/React/TypeScript + Tailwind (frontend), `golang.org/x/oauth2` + `google.golang.org/api/gmail/v1` (Gmail OAuth + API client).

**Spec:** `docs/superpowers/specs/2026-09-17-gmail-integration-design.md`

## Global Constraints

- Single user (super_admin) uses this app — no multi-account UI, no per-project mailbox scoping.
- Sync interval is exactly 3 hours; initial connect backfills the last 30 days.
- No email body or attachment bytes are stored locally — `emails` rows are metadata only; bodies/attachments are fetched live from Gmail per-request.
- Sync uses the Gmail History API (`users.history.list`) with a 404/history-expired fallback to an `after:`-date-query resync, per the spec's explicit technical choice.
- Use the official `google.golang.org/api/gmail/v1` client library, not raw HTTP calls to the Gmail REST API.
- Tokens (`access_token`, `refresh_token`) are stored as plaintext DB columns, matching the existing `billingo_settings.api_key` precedent — do not add new encryption infrastructure.
- New env vars: `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_OAUTH_REDIRECT_URL` (the user creates the actual Google Cloud OAuth credentials themselves — not part of this plan).
- New permission `gmail.manage`, granted to `super_admin` only; gates every Gmail/email/invoice-notice-sending route. The invoice-notice endpoint additionally reuses the existing `invoices.create` permission check (same access rule as issuing a real invoice).
- All new DB migrations ship both `.up.sql` and `.down.sql`, matching migrations 000019–000023.
- JSONB columns follow the `kanban.go` `Tags`/`TagsToJSON`/`TagsFromJSON` precedent: a Go `string` field tagged `gorm:"type:jsonb"` plus marshal/unmarshal helper functions — never a native slice/map field.
- Any real email actually sent during development/testing (compose, reply, or invoice notice) must go only to an address the user explicitly provides (e.g. their own) — never to a real client's address — per this project's standing rule against touching production-adjacent external state without explicit per-action approval.
- Testing philosophy (matches `docs/superpowers/plans/2026-09-17-billingo-invoicing.md`): this repo has no test-database convention, so DB-touching/API-calling code (handlers, the sync orchestrator, OAuth exchange) is verified via `go build`/`go vet` in the `devbridge_backend` container plus a manual smoke test — not automated tests. Only pure, DB-independent, side-effect-free functions get real Go `testing` unit tests (folder classification, dedup, history-expired detection, header parsing).
- All tooling (`go build`, `go vet`, `gofmt`, `go test`, `npx tsc --noEmit`) runs only inside the documented containers (`docker exec devbridge_backend ...`, `docker exec devbridge_frontend ...` or `docker-compose exec backend/frontend ...`) — never on the host.
- All user-facing frontend copy is in Hungarian.
- The Emails page gets deliberately polished/"extra" visual treatment (the user asked for this explicitly) — richer than this app's plainer CRUD screens — but must still reuse the existing Tailwind design tokens (`bg-card`, `text-muted-foreground`, `border-border`, etc.) rather than introducing a new design system.

---

## File Structure

**Backend — new files:**
- `backend/migrations/000024_add_gmail_integration.up.sql` / `.down.sql`
- `backend/internal/models/gmail_account.go`
- `backend/internal/models/email.go`
- `backend/internal/models/invoice_notice.go`
- `backend/internal/services/gmail_api.go` — `GmailAPI` interface + shared types (`GmailMessageMeta`, `GmailFullMessage`)
- `backend/internal/services/gmail_sync_helpers.go` — pure functions used by the sync job (unit tested)
- `backend/internal/services/gmail_sync_helpers_test.go`
- `backend/internal/services/gmail_oauth.go` — OAuth config, state map, auth URL, code exchange, token refresh
- `backend/internal/services/gmail_api_real.go` — `RealGmailAPI`, the real Gmail-SDK-backed implementation of `GmailAPI`
- `backend/internal/services/gmail_message_builder.go` — `BuildRawMessage` (RFC 2822 raw message for sending)
- `backend/internal/services/gmail_sync.go` — `RunGmailSync` orchestration + `StartGmailSyncScheduler`
- `backend/internal/handlers/gmail_auth_handler.go`
- `backend/internal/handlers/email_handler.go`
- `backend/internal/handlers/invoice_notice_handler.go`
- `backend/internal/routes/gmail_routes.go`
- `backend/internal/routes/email_routes.go`

**Backend — modified files:**
- `backend/go.mod` / `go.sum` (new dependencies)
- `backend/cmd/server/main.go` (start the sync scheduler)
- `backend/internal/routes/routes.go` (register the two new route groups)
- `backend/internal/routes/invoice_routes.go` (register the two invoice-notice routes)
- `backend/internal/handlers/invoice_handler.go` (`checkInvoiceAccess` becomes a package-level function so `invoice_notice_handler.go` can reuse it)

**Frontend — new files:**
- `frontend/src/services/gmailService.ts`
- `frontend/src/services/emailsService.ts`
- `frontend/src/app/dashboard/emails/page.tsx`

**Frontend — modified files:**
- `frontend/src/components/dashboard/DashboardNav.tsx` (new "E-mailek" link)
- `frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx` ("Értesítő küldése" button + status)

---

### Task 1: Migration — `gmail_accounts`, `emails`, `invoice_notices` + `gmail.manage` permission

**Files:**
- Create: `backend/migrations/000024_add_gmail_integration.up.sql`
- Create: `backend/migrations/000024_add_gmail_integration.down.sql`

**Interfaces:**
- Produces: tables `gmail_accounts`, `emails`, `invoice_notices`; permission row `gmail.manage`. Task 2's models must match these column names/types exactly.

- [ ] **Step 1: Write the up migration**

```sql
-- backend/migrations/000024_add_gmail_integration.up.sql
CREATE TABLE gmail_accounts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    email_address VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expiry TIMESTAMP NOT NULL,
    last_history_id VARCHAR(50) NOT NULL DEFAULT '',
    needs_reauth BOOLEAN NOT NULL DEFAULT false,
    last_synced_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE emails (
    id SERIAL PRIMARY KEY,
    gmail_account_id INTEGER NOT NULL REFERENCES gmail_accounts(id) ON DELETE CASCADE,
    gmail_message_id VARCHAR(100) NOT NULL,
    thread_id VARCHAR(100) NOT NULL DEFAULT '',
    folder VARCHAR(10) NOT NULL,
    from_address VARCHAR(255) NOT NULL DEFAULT '',
    from_name VARCHAR(255) NOT NULL DEFAULT '',
    to_addresses VARCHAR(1000) NOT NULL DEFAULT '',
    subject VARCHAR(998) NOT NULL DEFAULT '',
    snippet VARCHAR(1000) NOT NULL DEFAULT '',
    has_attachments BOOLEAN NOT NULL DEFAULT false,
    attachment_meta JSONB NOT NULL DEFAULT '[]',
    is_read BOOLEAN NOT NULL DEFAULT false,
    received_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (gmail_account_id, gmail_message_id)
);

CREATE INDEX idx_emails_account_folder_received ON emails (gmail_account_id, folder, received_at DESC);

CREATE TABLE invoice_notices (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
    period_start DATE,
    period_end DATE,
    gmail_message_id VARCHAR(100) NOT NULL,
    sent_by INTEGER NOT NULL REFERENCES users(id),
    sent_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoice_notices_project_period ON invoice_notices (project_id, period_start, period_end);

INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('gmail.manage', 'Manage Gmail Integration', 'Can connect the Gmail account and view/send synced emails', 'gmail', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'super_admin' AND p.name = 'gmail.manage';
```

- [ ] **Step 2: Write the down migration**

```sql
-- backend/migrations/000024_add_gmail_integration.down.sql
DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE name = 'gmail.manage');
DELETE FROM permissions WHERE name = 'gmail.manage';

DROP TABLE IF EXISTS invoice_notices;
DROP TABLE IF EXISTS emails;
DROP TABLE IF EXISTS gmail_accounts;
```

- [ ] **Step 3: Apply and verify the migration**

Run: `docker exec devbridge_backend go run ./cmd/server &` briefly, or simpler, restart the backend container so `database.RunMigrations` runs on boot:
```bash
docker-compose restart backend
docker logs devbridge_backend --tail 30
```
Expected: log shows migrations running with no error, and the new tables/permission exist:
```bash
docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "\d gmail_accounts" -c "\d emails" -c "\d invoice_notices" -c "SELECT name FROM permissions WHERE name = 'gmail.manage';"
```
Expected: all three `\d` commands print the columns above; the `SELECT` returns one row.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/000024_add_gmail_integration.up.sql backend/migrations/000024_add_gmail_integration.down.sql
git commit -m "feat(gmail): add gmail_accounts, emails, invoice_notices tables and gmail.manage permission"
```

---

### Task 2: Go models — `GmailAccount`, `Email`, `InvoiceNotice`

**Files:**
- Create: `backend/internal/models/gmail_account.go`
- Create: `backend/internal/models/email.go`
- Create: `backend/internal/models/invoice_notice.go`

**Interfaces:**
- Consumes: table schema from Task 1.
- Produces: `models.GmailAccount` (fields: `ID`, `UserID`, `EmailAddress`, `AccessToken`, `RefreshToken`, `TokenExpiry`, `LastHistoryID`, `NeedsReauth`, `LastSyncedAt *time.Time`, `CreatedAt`, `UpdatedAt`); `models.Email` (+ `models.EmailAttachmentMeta`, `models.AttachmentMetaToJSON([]EmailAttachmentMeta) string`, `(Email).Attachments() []EmailAttachmentMeta`); `models.InvoiceNotice`; response/request DTOs used by later handler tasks: `GmailStatusResponse`, `EmailListItem`, `EmailListResponse`, `EmailDetailResponse`, `EmailSendRequest`, `EmailSendResponse`, `InvoiceNoticeSendRequest`, `InvoiceNoticeResponse`, `InvoiceNoticeListResponse`.

- [ ] **Step 1: Create `gmail_account.go`**

```go
// backend/internal/models/gmail_account.go
package models

import "time"

type GmailAccount struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	UserID        uint       `json:"user_id" gorm:"not null;uniqueIndex"`
	EmailAddress  string     `json:"email_address" gorm:"size:255;not null"`
	AccessToken   string     `json:"-" gorm:"type:text;not null"`
	RefreshToken  string     `json:"-" gorm:"type:text;not null"`
	TokenExpiry   time.Time  `json:"-"`
	LastHistoryID string     `json:"-" gorm:"column:last_history_id;size:50"`
	NeedsReauth   bool       `json:"needs_reauth" gorm:"default:false"`
	LastSyncedAt  *time.Time `json:"last_synced_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (GmailAccount) TableName() string { return "gmail_accounts" }

type GmailStatusResponse struct {
	Success      bool       `json:"success"`
	Connected    bool       `json:"connected"`
	EmailAddress string     `json:"email_address,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	NeedsReauth  bool       `json:"needs_reauth,omitempty"`
}
```

- [ ] **Step 2: Create `email.go`**

```go
// backend/internal/models/email.go
package models

import (
	"encoding/json"
	"time"
)

type EmailAttachmentMeta struct {
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	AttachmentID string `json:"attachment_id"`
}

type Email struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	GmailAccountID uint      `json:"gmail_account_id" gorm:"not null"`
	GmailMessageID string    `json:"gmail_message_id" gorm:"size:100;not null"`
	ThreadID       string    `json:"thread_id" gorm:"size:100"`
	Folder         string    `json:"folder" gorm:"size:10;not null"` // "inbox" | "sent"
	FromAddress    string    `json:"from_address" gorm:"size:255"`
	FromName       string    `json:"from_name" gorm:"size:255"`
	ToAddresses    string    `json:"to_addresses" gorm:"size:1000"`
	Subject        string    `json:"subject" gorm:"size:998"`
	Snippet        string    `json:"snippet" gorm:"size:1000"`
	HasAttachments bool      `json:"has_attachments"`
	AttachmentMeta string    `json:"-" gorm:"type:jsonb;default:'[]'"`
	IsRead         bool      `json:"is_read"`
	ReceivedAt     time.Time `json:"received_at"`
	SyncedAt       time.Time `json:"synced_at"`
}

func (Email) TableName() string { return "emails" }

// Attachments unmarshals AttachmentMeta, following the Tags/TagsFromJSON
// precedent in models/kanban.go. Returns nil (not an error) on bad JSON,
// since AttachmentMeta is always written by AttachmentMetaToJSON.
func (e Email) Attachments() []EmailAttachmentMeta {
	var out []EmailAttachmentMeta
	_ = json.Unmarshal([]byte(e.AttachmentMeta), &out)
	return out
}

func AttachmentMetaToJSON(items []EmailAttachmentMeta) string {
	if len(items) == 0 {
		return "[]"
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

type EmailListItem struct {
	ID             uint                  `json:"id"`
	Folder         string                `json:"folder"`
	FromAddress    string                `json:"from_address"`
	FromName       string                `json:"from_name"`
	ToAddresses    string                `json:"to_addresses"`
	Subject        string                `json:"subject"`
	Snippet        string                `json:"snippet"`
	HasAttachments bool                  `json:"has_attachments"`
	Attachments    []EmailAttachmentMeta `json:"attachments"`
	IsRead         bool                  `json:"is_read"`
	ReceivedAt     time.Time             `json:"received_at"`
}

type EmailListResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Emails  []EmailListItem `json:"emails,omitempty"`
	Total   int64           `json:"total,omitempty"`
}

type EmailDetailResponse struct {
	Success     bool                  `json:"success"`
	Message     string                `json:"message,omitempty"`
	ID          uint                  `json:"id,omitempty"`
	Folder      string                `json:"folder,omitempty"`
	Subject     string                `json:"subject,omitempty"`
	From        string                `json:"from,omitempty"`
	To          string                `json:"to,omitempty"`
	BodyText    string                `json:"body_text,omitempty"`
	BodyHTML    string                `json:"body_html,omitempty"`
	Attachments []EmailAttachmentMeta `json:"attachments,omitempty"`
	ReceivedAt  time.Time             `json:"received_at,omitempty"`
}

type EmailSendRequest struct {
	To               string `json:"to"`
	Subject          string `json:"subject"`
	Body             string `json:"body"`
	InReplyToEmailID uint   `json:"in_reply_to_email_id,omitempty"` // local emails.id being replied to
}

type EmailSendResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message,omitempty"`
	GmailMessageID string `json:"gmail_message_id,omitempty"`
}
```

- [ ] **Step 3: Create `invoice_notice.go`**

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
}

func (InvoiceNotice) TableName() string { return "invoice_notices" }

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
```

- [ ] **Step 4: Build to verify**

Run: `docker exec devbridge_backend go build ./...`
Expected: exits 0, no compile errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/gmail_account.go backend/internal/models/email.go backend/internal/models/invoice_notice.go
git commit -m "feat(gmail): add GmailAccount, Email, InvoiceNotice models"
```

---

### Task 3: Add Gmail/OAuth Go dependencies

**Files:**
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Produces: importable packages `golang.org/x/oauth2`, `golang.org/x/oauth2/google`, `google.golang.org/api/gmail/v1`, `google.golang.org/api/option` used by Tasks 5–7.

- [ ] **Step 1: Add the dependencies**

Run inside the container (host Go toolchain must never be used, per Global Constraints):
```bash
docker exec devbridge_backend go get golang.org/x/oauth2@latest google.golang.org/api@latest
```
Expected: `go.mod`/`go.sum` updated with `golang.org/x/oauth2` and `google.golang.org/api` (which provides both `gmail/v1` and `option`) plus their transitive indirect deps.

- [ ] **Step 2: Verify the module builds**

Run: `docker exec devbridge_backend go build ./...`
Expected: exits 0 (no code uses the new packages yet, so this just confirms the dependency resolves and downloads cleanly).

- [ ] **Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore(gmail): add oauth2 and google-api-go-client gmail dependencies"
```

---

### Task 4: `GmailAPI` interface + pure sync helper functions (with unit tests)

**Files:**
- Create: `backend/internal/services/gmail_api.go`
- Create: `backend/internal/services/gmail_sync_helpers.go`
- Create: `backend/internal/services/gmail_sync_helpers_test.go`

**Interfaces:**
- Consumes: `models.EmailAttachmentMeta` (Task 2).
- Produces: `GmailMessageMeta` struct, `GmailFullMessage` struct, `GmailAPI` interface (implemented for real in Task 6, and implicitly fakeable in tests); helper functions `classifyFolder([]string) (string, bool)`, `isHistoryExpiredError(error) bool`, `isAuthError(error) bool`, `pendingMessageIDs([]string, map[string]bool) []string`, `splitNameAddress(string) (string, string)`, `containsLabel([]string, string) bool` — all consumed by Task 5/6/7.

- [ ] **Step 1: Write the `GmailAPI` interface and shared types**

```go
// backend/internal/services/gmail_api.go
package services

import (
	"context"
	"time"

	"dev-bridge-manager/internal/models"
)

// GmailMessageMeta is the metadata-only shape stored per synced email.
type GmailMessageMeta struct {
	GmailMessageID string
	ThreadID       string
	Folder         string // "inbox" | "sent"
	FromAddress    string
	FromName       string
	ToAddresses    string
	Subject        string
	Snippet        string
	IsRead         bool
	ReceivedAt     time.Time
	Attachments    []models.EmailAttachmentMeta
}

// GmailFullMessage is fetched live (never persisted) when a user opens an
// email or replies to one.
type GmailFullMessage struct {
	GmailMessageMeta
	BodyText         string
	BodyHTML         string
	MessageIDHeader  string
	ReferencesHeader string
}

// GmailAPI is the seam between the sync/handler logic and the real Gmail
// SDK, so gmail_sync.go and email_handler.go can be exercised with a fake
// in tests without hitting Google's servers.
type GmailAPI interface {
	ListMessageIDs(ctx context.Context, account *models.GmailAccount, query string) ([]string, error)
	GetMessageMetadata(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailMessageMeta, error)
	GetFullMessage(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailFullMessage, error)
	GetProfileHistoryID(ctx context.Context, account *models.GmailAccount) (string, error)
	ListHistory(ctx context.Context, account *models.GmailAccount, startHistoryID string) (added []string, deleted []string, newHistoryID string, err error)
	GetAttachment(ctx context.Context, account *models.GmailAccount, messageID, attachmentID string) (data []byte, err error)
	SendMessage(ctx context.Context, account *models.GmailAccount, raw []byte) (gmailMessageID string, err error)
}
```

- [ ] **Step 2: Write the failing tests for the pure helpers**

```go
// backend/internal/services/gmail_sync_helpers_test.go
package services

import (
	"errors"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestClassifyFolder(t *testing.T) {
	cases := []struct {
		name       string
		labels     []string
		wantFolder string
		wantOK     bool
	}{
		{"inbox", []string{"INBOX", "UNREAD"}, "inbox", true},
		{"sent", []string{"SENT"}, "sent", true},
		{"sent takes priority over inbox", []string{"INBOX", "SENT"}, "sent", true},
		{"draft only is skipped", []string{"DRAFT"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			folder, ok := classifyFolder(tc.labels)
			if folder != tc.wantFolder || ok != tc.wantOK {
				t.Errorf("classifyFolder(%v) = (%q, %v), want (%q, %v)", tc.labels, folder, ok, tc.wantFolder, tc.wantOK)
			}
		})
	}
}

func TestPendingMessageIDs(t *testing.T) {
	fetched := []string{"a", "b", "c"}
	existing := map[string]bool{"b": true}

	got := pendingMessageIDs(fetched, existing)

	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("pendingMessageIDs = %v, want [a c]", got)
	}
}

func TestIsHistoryExpiredError(t *testing.T) {
	notFound := &googleapi.Error{Code: 404}
	if !isHistoryExpiredError(notFound) {
		t.Error("expected 404 googleapi.Error to be treated as history-expired")
	}

	forbidden := &googleapi.Error{Code: 403}
	if isHistoryExpiredError(forbidden) {
		t.Error("expected 403 googleapi.Error not to be treated as history-expired")
	}

	if isHistoryExpiredError(errors.New("plain error")) {
		t.Error("expected a non-googleapi error to be false")
	}
}

func TestIsAuthError(t *testing.T) {
	unauthorized := &googleapi.Error{Code: 401}
	if !isAuthError(unauthorized) {
		t.Error("expected 401 googleapi.Error to be treated as an auth error")
	}
	if isAuthError(errors.New("plain error")) {
		t.Error("expected a non-googleapi error to be false")
	}
}

func TestSplitNameAddress(t *testing.T) {
	name, addr := splitNameAddress(`"Jane Doe" <jane@example.com>`)
	if name != "Jane Doe" || addr != "jane@example.com" {
		t.Errorf("got (%q, %q)", name, addr)
	}

	name, addr = splitNameAddress("plain@example.com")
	if name != "" || addr != "plain@example.com" {
		t.Errorf("got (%q, %q), want (\"\", plain@example.com)", name, addr)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run 'TestClassifyFolder|TestPendingMessageIDs|TestIsHistoryExpiredError|TestIsAuthError|TestSplitNameAddress' -v`
Expected: FAIL — `classifyFolder`, `pendingMessageIDs`, `isHistoryExpiredError`, `isAuthError`, `splitNameAddress` undefined.

- [ ] **Step 4: Implement the pure helpers**

```go
// backend/internal/services/gmail_sync_helpers.go
package services

import (
	"errors"
	"strings"

	"dev-bridge-manager/internal/models"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

// classifyFolder maps a Gmail message's labelIds to this app's two-folder
// model. A message with both SENT and INBOX (e.g. replying to yourself)
// is classified as "sent" since that's the action the user took. Messages
// with neither label (drafts, pure spam/trash) are skipped.
func classifyFolder(labelIDs []string) (folder string, ok bool) {
	hasInbox, hasSent := false, false
	for _, l := range labelIDs {
		if l == "INBOX" {
			hasInbox = true
		}
		if l == "SENT" {
			hasSent = true
		}
	}
	switch {
	case hasSent:
		return "sent", true
	case hasInbox:
		return "inbox", true
	default:
		return "", false
	}
}

// pendingMessageIDs returns the ids in fetched that are not already in
// existingIDs, preserving fetched's order. Used by the sync job to avoid
// re-fetching/re-inserting messages already mirrored locally.
func pendingMessageIDs(fetched []string, existingIDs map[string]bool) []string {
	var out []string
	for _, id := range fetched {
		if !existingIDs[id] {
			out = append(out, id)
		}
	}
	return out
}

// isHistoryExpiredError reports whether err is Gmail's 404 for a
// startHistoryId that's aged out of Gmail's history buffer, which requires
// falling back to a date-query resync.
func isHistoryExpiredError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}

// isAuthError reports whether err means the stored refresh token is no
// longer valid (revoked access), which should mark the account as needing
// the user to reconnect rather than retrying.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == 401
	}
	return false
}

// splitNameAddress parses a From/To header value like `"Jane Doe" <jane@x.com>`
// into (name, address). If there's no display name, name is "".
func splitNameAddress(header string) (name, address string) {
	header = strings.TrimSpace(header)
	if idx := strings.LastIndex(header, "<"); idx != -1 && strings.HasSuffix(header, ">") {
		name = strings.Trim(header[:idx], ` "`)
		address = strings.TrimSuffix(header[idx+1:], ">")
		return name, address
	}
	return "", header
}

func containsLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// extractAttachmentMeta walks a message payload's parts recursively,
// collecting parts that are real attachments (have a filename and an
// attachment id) as opposed to inline text/html body parts.
func extractAttachmentMeta(part *gmail.MessagePart) []models.EmailAttachmentMeta {
	if part == nil {
		return nil
	}
	var out []models.EmailAttachmentMeta
	if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
		out = append(out, models.EmailAttachmentMeta{
			Filename:     part.Filename,
			Size:         part.Body.Size,
			AttachmentID: part.Body.AttachmentId,
		})
	}
	for _, child := range part.Parts {
		out = append(out, extractAttachmentMeta(child)...)
	}
	return out
}

// extractBody walks a message payload for the first text/plain and
// text/html parts. Gmail's payload body data is base64url-encoded.
func extractBody(part *gmail.MessagePart) (text, html string) {
	if part == nil {
		return "", ""
	}
	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		text = decodeBase64URL(part.Body.Data)
	}
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
		html = decodeBase64URL(part.Body.Data)
	}
	for _, child := range part.Parts {
		childText, childHTML := extractBody(child)
		if text == "" {
			text = childText
		}
		if html == "" {
			html = childHTML
		}
	}
	return text, html
}
```

- [ ] **Step 5: Add the base64url decode helper**

```go
// Add to backend/internal/services/gmail_sync_helpers.go, alongside extractBody:

func decodeBase64URL(s string) string {
	b, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}
```

Add `"encoding/base64"` to the file's import block.

- [ ] **Step 6: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run 'TestClassifyFolder|TestPendingMessageIDs|TestIsHistoryExpiredError|TestIsAuthError|TestSplitNameAddress' -v`
Expected: all PASS.

- [ ] **Step 7: Build the whole module**

Run: `docker exec devbridge_backend go build ./...`
Expected: exits 0.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/gmail_api.go backend/internal/services/gmail_sync_helpers.go backend/internal/services/gmail_sync_helpers_test.go
git commit -m "feat(gmail): add GmailAPI interface and pure sync helper functions"
```

---

### Task 5: OAuth config, state map, auth URL, code exchange, token refresh

**Files:**
- Create: `backend/internal/services/gmail_oauth.go`

**Interfaces:**
- Consumes: `models.GmailAccount` (Task 2); env vars `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_OAUTH_REDIRECT_URL`.
- Produces: `GmailAuthURL(userID uint) string`, `ExchangeAndSaveGmailAccount(ctx context.Context, code, state string) error`, `validTokenForAccount(ctx context.Context, account *models.GmailAccount) (*oauth2.Token, error)` — all consumed by Task 6 (`gmail_api_real.go`) and Task 8 (`gmail_auth_handler.go`).

- [ ] **Step 1: Write `gmail_oauth.go`**

```go
// backend/internal/services/gmail_oauth.go
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var gmailScopes = []string{gmail.GmailReadonlyScope, gmail.GmailSendScope}

func gmailOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		Scopes:       gmailScopes,
		Endpoint:     google.Endpoint,
	}
}

// The OAuth "state" parameter is this flow's CSRF protection: auth-url
// (called with the app's normal JWT auth) mints a one-time random state
// tied to the requesting user; the callback (hit by Google's plain browser
// redirect, which carries no app auth header) trades it back for that
// user id. Since this app runs as a single process with no horizontal
// scaling, an in-memory map is enough — a server restart between auth-url
// and callback just means the user clicks "Connect Gmail" again.
type oauthStateEntry struct {
	userID    uint
	expiresAt time.Time
}

var (
	oauthStateMu      sync.Mutex
	oauthStatePending = map[string]oauthStateEntry{}
)

func newOAuthState(userID uint) string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	state := hex.EncodeToString(buf)

	oauthStateMu.Lock()
	oauthStatePending[state] = oauthStateEntry{userID: userID, expiresAt: time.Now().Add(10 * time.Minute)}
	oauthStateMu.Unlock()
	return state
}

// consumeOAuthState validates and deletes a one-time state value, returning
// the userID it was issued for. Returns an error for an unknown or expired
// state (forged or stale callback).
func consumeOAuthState(state string) (uint, error) {
	oauthStateMu.Lock()
	defer oauthStateMu.Unlock()

	entry, ok := oauthStatePending[state]
	delete(oauthStatePending, state)
	if !ok {
		return 0, fmt.Errorf("unknown or already-used oauth state")
	}
	if time.Now().After(entry.expiresAt) {
		return 0, fmt.Errorf("oauth state expired")
	}
	return entry.userID, nil
}

// GmailAuthURL builds the Google consent screen URL for userID to connect
// their Gmail account. AccessTypeOffline + prompt=consent ensure Google
// always returns a refresh_token, even on a repeat connect.
func GmailAuthURL(userID uint) string {
	state := newOAuthState(userID)
	return gmailOAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

// ExchangeAndSaveGmailAccount trades an OAuth code for tokens, fetches the
// connected address, and upserts the gmail_accounts row for state's user.
func ExchangeAndSaveGmailAccount(ctx context.Context, code, state string) error {
	userID, err := consumeOAuthState(state)
	if err != nil {
		return err
	}

	token, err := gmailOAuthConfig().Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange oauth code: %w", err)
	}

	svc, err := gmail.NewService(ctx, option.WithTokenSource(gmailOAuthConfig().TokenSource(ctx, token)))
	if err != nil {
		return fmt.Errorf("failed to create gmail service: %w", err)
	}
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return fmt.Errorf("failed to fetch gmail profile: %w", err)
	}

	account := models.GmailAccount{
		UserID:       userID,
		EmailAddress: profile.EmailAddress,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry,
		NeedsReauth:  false,
	}

	return database.GetDB().
		Where("user_id = ?", userID).
		Assign(account).
		FirstOrCreate(&account).Error
}

// validTokenForAccount returns a non-expired oauth2.Token for account,
// refreshing and persisting it first if the stored access token has
// expired. Every RealGmailAPI call goes through this so a refreshed token
// is never silently dropped after a single request.
func validTokenForAccount(ctx context.Context, account *models.GmailAccount) (*oauth2.Token, error) {
	existing := &oauth2.Token{
		AccessToken:  account.AccessToken,
		RefreshToken: account.RefreshToken,
		Expiry:       account.TokenExpiry,
	}

	fresh, err := gmailOAuthConfig().TokenSource(ctx, existing).Token()
	if err != nil {
		return nil, err
	}

	if fresh.AccessToken != existing.AccessToken {
		database.GetDB().Model(account).Updates(map[string]interface{}{
			"access_token": fresh.AccessToken,
			"token_expiry": fresh.Expiry,
		})
		account.AccessToken = fresh.AccessToken
		account.TokenExpiry = fresh.Expiry
	}
	return fresh, nil
}
```

- [ ] **Step 2: Build to verify**

Run: `docker exec devbridge_backend go build ./...`
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/gmail_oauth.go
git commit -m "feat(gmail): add OAuth config, one-time state CSRF protection, and token refresh"
```

---

### Task 6: `RealGmailAPI` — real Gmail SDK implementation

**Files:**
- Create: `backend/internal/services/gmail_api_real.go`
- Create: `backend/internal/services/gmail_message_builder.go`

**Interfaces:**
- Consumes: `GmailAPI` interface, `GmailMessageMeta`/`GmailFullMessage` (Task 4); `classifyFolder`, `extractAttachmentMeta`, `extractBody`, `splitNameAddress`, `containsLabel`, `decodeBase64URL` (Task 4); `validTokenForAccount`, `gmailOAuthConfig` (Task 5).
- Produces: `NewRealGmailAPI() *RealGmailAPI` (implements `GmailAPI`), `BuildRawMessage(fromAddress, to, subject, body, inReplyToHeader, referencesHeader string) []byte` — consumed by Task 10 (`email_handler.go`) and Task 11 (`invoice_notice_handler.go`).

- [ ] **Step 1: Write `RealGmailAPI`**

```go
// backend/internal/services/gmail_api_real.go
package services

import (
	"context"
	"encoding/base64"
	"strconv"

	"dev-bridge-manager/internal/models"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type RealGmailAPI struct{}

func NewRealGmailAPI() *RealGmailAPI { return &RealGmailAPI{} }

func (a *RealGmailAPI) serviceFor(ctx context.Context, account *models.GmailAccount) (*gmail.Service, error) {
	token, err := validTokenForAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	return gmail.NewService(ctx, option.WithTokenSource(gmailOAuthConfig().TokenSource(ctx, token)))
}

func (a *RealGmailAPI) ListMessageIDs(ctx context.Context, account *models.GmailAccount, query string) ([]string, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	var ids []string
	pageToken := ""
	for {
		call := svc.Users.Messages.List("me").Q(query).MaxResults(500)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, m := range resp.Messages {
			ids = append(ids, m.Id)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return ids, nil
}

func (a *RealGmailAPI) GetMessageMetadata(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailMessageMeta, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	msg, err := svc.Users.Messages.Get("me", messageID).Format("metadata").
		MetadataHeaders("From", "To", "Subject").Do()
	if err != nil {
		return nil, err
	}

	folder, ok := classifyFolder(msg.LabelIds)
	if !ok {
		return nil, nil // caller (gmail_sync.go) skips messages with no INBOX/SENT label
	}

	meta := &GmailMessageMeta{
		GmailMessageID: msg.Id,
		ThreadID:       msg.ThreadId,
		Folder:         folder,
		Snippet:        msg.Snippet,
		IsRead:         !containsLabel(msg.LabelIds, "UNREAD"),
		ReceivedAt:     msTimeToTime(msg.InternalDate),
	}
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			meta.FromName, meta.FromAddress = splitNameAddress(h.Value)
		case "To":
			meta.ToAddresses = h.Value
		case "Subject":
			meta.Subject = h.Value
		}
	}
	meta.Attachments = extractAttachmentMeta(msg.Payload)
	return meta, nil
}

func (a *RealGmailAPI) GetFullMessage(ctx context.Context, account *models.GmailAccount, messageID string) (*GmailFullMessage, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	msg, err := svc.Users.Messages.Get("me", messageID).Format("full").Do()
	if err != nil {
		return nil, err
	}

	folder, _ := classifyFolder(msg.LabelIds)
	full := &GmailFullMessage{
		GmailMessageMeta: GmailMessageMeta{
			GmailMessageID: msg.Id,
			ThreadID:       msg.ThreadId,
			Folder:         folder,
			Snippet:        msg.Snippet,
			ReceivedAt:     msTimeToTime(msg.InternalDate),
			Attachments:    extractAttachmentMeta(msg.Payload),
		},
	}
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			full.FromName, full.FromAddress = splitNameAddress(h.Value)
		case "To":
			full.ToAddresses = h.Value
		case "Subject":
			full.Subject = h.Value
		case "Message-ID":
			full.MessageIDHeader = h.Value
		case "References":
			full.ReferencesHeader = h.Value
		}
	}
	full.BodyText, full.BodyHTML = extractBody(msg.Payload)
	return full, nil
}

func (a *RealGmailAPI) GetProfileHistoryID(ctx context.Context, account *models.GmailAccount) (string, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return "", err
	}
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(profile.HistoryId, 10), nil
}

func (a *RealGmailAPI) ListHistory(ctx context.Context, account *models.GmailAccount, startHistoryID string) (added, deleted []string, newHistoryID string, err error) {
	svc, svcErr := a.serviceFor(ctx, account)
	if svcErr != nil {
		return nil, nil, "", svcErr
	}

	startID, parseErr := strconv.ParseUint(startHistoryID, 10, 64)
	if parseErr != nil {
		return nil, nil, "", parseErr
	}

	newHistoryID = startHistoryID
	pageToken := ""
	for {
		call := svc.Users.History.List("me").StartHistoryId(startID).
			HistoryTypes("messageAdded", "messageDeleted")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, callErr := call.Do()
		if callErr != nil {
			return nil, nil, "", callErr
		}

		for _, h := range resp.History {
			for _, m := range h.MessagesAdded {
				added = append(added, m.Message.Id)
			}
			for _, m := range h.MessagesDeleted {
				deleted = append(deleted, m.Message.Id)
			}
		}
		if resp.HistoryId != 0 {
			newHistoryID = strconv.FormatUint(resp.HistoryId, 10)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return added, deleted, newHistoryID, nil
}

func (a *RealGmailAPI) GetAttachment(ctx context.Context, account *models.GmailAccount, messageID, attachmentID string) ([]byte, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return nil, err
	}

	att, err := svc.Users.Messages.Attachments.Get("me", messageID, attachmentID).Do()
	if err != nil {
		return nil, err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(att.Data)
}

func (a *RealGmailAPI) SendMessage(ctx context.Context, account *models.GmailAccount, raw []byte) (string, error) {
	svc, err := a.serviceFor(ctx, account)
	if err != nil {
		return "", err
	}

	msg := &gmail.Message{Raw: base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(raw)}
	sent, err := svc.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return "", err
	}
	return sent.Id, nil
}

func msTimeToTime(ms int64) time.Time {
	return time.UnixMilli(ms)
}
```

Add `"time"` to the import block (Gmail's `InternalDate` is milliseconds since epoch).

- [ ] **Step 2: Write `BuildRawMessage`**

```go
// backend/internal/services/gmail_message_builder.go
package services

import (
	"fmt"
	"strings"
)

// BuildRawMessage builds an RFC 2822 message ready for GmailAPI.SendMessage.
// When inReplyToHeader is non-empty, In-Reply-To/References headers are set
// so Gmail threads the reply under the original message.
func BuildRawMessage(fromAddress, to, subject, body, inReplyToHeader, referencesHeader string) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	if inReplyToHeader != "" {
		sb.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", inReplyToHeader))
		refs := strings.TrimSpace(referencesHeader + " " + inReplyToHeader)
		sb.WriteString(fmt.Sprintf("References: %s\r\n", refs))
	}
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	sb.WriteString(body)
	return []byte(sb.String())
}
```

- [ ] **Step 3: Build to verify**

Run: `docker exec devbridge_backend go build ./...`
Expected: exits 0. (`go vet ./...` should also be run and be clean.)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/gmail_api_real.go backend/internal/services/gmail_message_builder.go
git commit -m "feat(gmail): implement RealGmailAPI against the official Gmail SDK"
```

---

### Task 7: Sync orchestration + scheduler, wired into `main.go`

**Files:**
- Create: `backend/internal/services/gmail_sync.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `GmailAPI`, `pendingMessageIDs`, `isHistoryExpiredError`, `isAuthError` (Task 4); `models.GmailAccount`, `models.Email`, `models.AttachmentMetaToJSON` (Task 2); `NewRealGmailAPI` (Task 6).
- Produces: `RunGmailSync(api GmailAPI)`, `StartGmailSyncScheduler()` — the latter consumed by `main.go`.

- [ ] **Step 1: Write `gmail_sync.go`**

```go
// backend/internal/services/gmail_sync.go
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const gmailSyncInterval = 3 * time.Hour
const gmailInitialBackfillDays = 30

// StartGmailSyncScheduler mirrors services.StartAutoInvoiceScheduler's plain
// time.Ticker pattern: no cron dependency exists in this codebase, and a
// 3-hour cadence tolerates the occasional missed tick from a restart.
func StartGmailSyncScheduler() {
	api := NewRealGmailAPI()
	RunGmailSync(api)

	ticker := time.NewTicker(gmailSyncInterval)
	for range ticker.C {
		RunGmailSync(api)
	}
}

func RunGmailSync(api GmailAPI) {
	var accounts []models.GmailAccount
	if err := database.GetDB().Find(&accounts).Error; err != nil {
		log.Printf("gmail sync: failed to load accounts: %v", err)
		return
	}
	for i := range accounts {
		if err := syncAccount(context.Background(), api, &accounts[i]); err != nil {
			log.Printf("gmail sync: account %d failed: %v", accounts[i].ID, err)
		}
	}
}

func syncAccount(ctx context.Context, api GmailAPI, account *models.GmailAccount) error {
	db := database.GetDB()

	if account.LastHistoryID == "" {
		cutoff := time.Now().AddDate(0, 0, -gmailInitialBackfillDays)
		if err := backfillAccount(ctx, api, db, account, cutoff); err != nil {
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
				if err := backfillAccount(ctx, api, db, account, cutoff); err != nil {
					return recordSyncFailure(db, account, err)
				}
			} else {
				return recordSyncFailure(db, account, err)
			}
		} else {
			if err := applyAddedMessages(ctx, api, db, account, added); err != nil {
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
	return nil
}

// recordSyncFailure flags the account for reconnect when the failure is an
// auth error, then returns err unchanged so the caller still logs it.
func recordSyncFailure(db *gorm.DB, account *models.GmailAccount, err error) error {
	if isAuthError(err) {
		db.Model(account).Update("needs_reauth", true)
	}
	return err
}

func backfillAccount(ctx context.Context, api GmailAPI, db *gorm.DB, account *models.GmailAccount, after time.Time) error {
	query := fmt.Sprintf("after:%d", after.Unix())
	ids, err := api.ListMessageIDs(ctx, account, query)
	if err != nil {
		return err
	}

	if err := applyAddedMessages(ctx, api, db, account, ids); err != nil {
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

func applyAddedMessages(ctx context.Context, api GmailAPI, db *gorm.DB, account *models.GmailAccount, ids []string) error {
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

		db.Create(&models.Email{
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
		})
	}
	return nil
}
```

- [ ] **Step 2: Wire the scheduler into `main.go`**

In `backend/cmd/server/main.go`, right after the existing auto-invoice scheduler line:

```go
	// Monthly auto-invoicing for hourly projects with it enabled (see
	// services.RunAutoInvoicing)
	go services.StartAutoInvoiceScheduler()

	// Gmail inbox/sent mirror sync, every 3 hours (see services.RunGmailSync)
	go services.StartGmailSyncScheduler()
```

- [ ] **Step 3: Build to verify**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: both exit 0.

- [ ] **Step 4: Restart and smoke-test with no connected accounts**

```bash
docker-compose restart backend
docker logs devbridge_backend --tail 30
```
Expected: no panics/errors; `RunGmailSync` runs once at startup, finds zero `gmail_accounts` rows, and returns immediately (no OAuth calls happen yet since Task 8 hasn't shipped the connect flow).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/gmail_sync.go backend/cmd/server/main.go
git commit -m "feat(gmail): add History-API sync job with 30-day backfill and 3h scheduler"
```

---

### Task 8: OAuth handler + routes (connect/callback/status/disconnect)

**Files:**
- Create: `backend/internal/handlers/gmail_auth_handler.go`
- Create: `backend/internal/routes/gmail_routes.go`
- Modify: `backend/internal/routes/routes.go`

**Interfaces:**
- Consumes: `services.GmailAuthURL`, `services.ExchangeAndSaveGmailAccount` (Task 5); `models.GmailStatusResponse` (Task 2); `middleware.JWTMiddleware`, `middleware.RequirePermission` (existing).
- Produces: routes `GET /api/v1/gmail/auth-url`, `GET /api/v1/gmail/callback`, `GET /api/v1/gmail/status`, `POST /api/v1/gmail/disconnect` — consumed by Task 12 (`gmailService.ts`).

- [ ] **Step 1: Write the handler**

```go
// backend/internal/handlers/gmail_auth_handler.go
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
```

- [ ] **Step 2: Write the routes**

```go
// backend/internal/routes/gmail_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupGmailRoutes(api fiber.Router) {
	h := handlers.NewGmailAuthHandler()
	gmail := api.Group("/gmail")

	// Public: Google's browser redirect lands here directly, carrying no
	// app Authorization header. Protected by the one-time state param
	// (see services.consumeOAuthState) instead of JWT.
	gmail.Get("/callback", h.Callback)

	gmail.Use(middleware.JWTMiddleware())
	gmail.Get("/auth-url", middleware.RequirePermission("gmail.manage"), h.AuthURL)
	gmail.Get("/status", middleware.RequirePermission("gmail.manage"), h.Status)
	gmail.Post("/disconnect", middleware.RequirePermission("gmail.manage"), h.Disconnect)
}
```

- [ ] **Step 3: Register the route group**

In `backend/internal/routes/routes.go`, add after `SetupBillingoSettingsRoutes(v1)`:

```go
	SetupBillingoSettingsRoutes(v1)  // Billingo settings endpoints
	SetupGmailRoutes(v1)             // Gmail OAuth connect/status endpoints
```

- [ ] **Step 4: Build to verify**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: both exit 0.

- [ ] **Step 5: Manual smoke test**

```bash
docker-compose restart backend
curl -s http://localhost:8080/api/v1/gmail/status -H "Authorization: Bearer <a real JWT for the super_admin user>"
```
Expected: `{"success":true,"connected":false}`.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/gmail_auth_handler.go backend/internal/routes/gmail_routes.go backend/internal/routes/routes.go
git commit -m "feat(gmail): add OAuth connect/callback/status/disconnect endpoints"
```

---

### Task 9: Refactor `checkInvoiceAccess` into a shared package-level function

**Files:**
- Modify: `backend/internal/handlers/invoice_handler.go`

**Interfaces:**
- Produces: `checkInvoiceAccess(ps *services.PermissionService, userID uint, action string) error` (package-level, was a method) — consumed by Task 11 (`invoice_notice_handler.go`).

- [ ] **Step 1: Change the method to a package-level function**

In `backend/internal/handlers/invoice_handler.go`, replace:

```go
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
```

with:

```go
// checkInvoiceAccess - jogosultság ellenőrzése egy adott invoices.* jogra,
// admin/super_admin/manager fallback-kel. Package-level (not a method) so
// invoice_notice_handler.go can reuse the exact same rule without
// duplicating it or depending on an *InvoiceHandler instance.
func checkInvoiceAccess(ps *services.PermissionService, userID uint, action string) error {
	hasPermission, err := ps.CheckUserPermission(userID, action)
	if err != nil || !hasPermission {
		user, err := ps.GetUserWithPermissions(userID)
		if err != nil {
			return fiber.NewError(500, "Error checking permissions")
		}
		if user.Role.Name != "admin" && user.Role.Name != "super_admin" && user.Role.Name != "manager" {
			return fiber.NewError(403, "Insufficient permissions")
		}
	}
	return nil
}
```

- [ ] **Step 2: Update all call sites in the same file**

Replace every occurrence of `h.checkInvoiceAccess(currentUserID, "...")` in `invoice_handler.go` (lines 137, 468, 487, 506, 583, 628, 672 as of this plan's writing) with `checkInvoiceAccess(h.permissionService, currentUserID, "...")`, keeping each call's `action` string unchanged.

- [ ] **Step 3: Build to verify**

Run: `docker exec devbridge_backend go build ./...`
Expected: exits 0 — confirms no other file referenced the old method form and every call site was updated.

- [ ] **Step 4: Manual smoke test of an existing invoice endpoint**

```bash
docker-compose restart backend
curl -s http://localhost:8080/api/v1/projects/1/invoices -H "Authorization: Bearer <a real JWT>"
```
Expected: same successful response shape as before this refactor (a permissions-only refactor, no behavior change).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/invoice_handler.go
git commit -m "refactor(invoices): make checkInvoiceAccess a package-level function"
```

---

### Task 10: Email handlers (list/get/attachment/send) + routes

**Files:**
- Create: `backend/internal/handlers/email_handler.go`
- Create: `backend/internal/routes/email_routes.go`
- Modify: `backend/internal/routes/routes.go`

**Interfaces:**
- Consumes: `models.Email`, `models.EmailListResponse`, `models.EmailListItem`, `models.EmailDetailResponse`, `models.EmailSendRequest`, `models.EmailSendResponse` (Task 2); `services.GmailAPI`, `services.NewRealGmailAPI`, `services.BuildRawMessage` (Task 4/6); `models.GmailAccount` (Task 2).
- Produces: routes `GET /api/v1/emails`, `GET /api/v1/emails/:id`, `GET /api/v1/emails/:id/attachments/:attachmentId`, `POST /api/v1/emails/send` — consumed by Task 12 (`emailsService.ts`).

- [ ] **Step 1: Write the handler**

```go
// backend/internal/handlers/email_handler.go
package handlers

import (
	"mime"
	"path/filepath"
	"strconv"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type EmailHandler struct {
	gmailAPI services.GmailAPI
}

func NewEmailHandler() *EmailHandler {
	return &EmailHandler{gmailAPI: services.NewRealGmailAPI()}
}

func currentGmailAccount(userID uint) (*models.GmailAccount, error) {
	var account models.GmailAccount
	if err := database.GetDB().Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// ListEmails - GET /api/v1/emails?folder=inbox|sent&page=
func (h *EmailHandler) ListEmails(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailListResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	folder := c.Query("folder", "inbox")
	if folder != "inbox" && folder != "sent" {
		return c.Status(400).JSON(models.EmailListResponse{Success: false, Message: "folder must be 'inbox' or 'sent'"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	const pageSize = 25

	var total int64
	db := database.GetDB().Model(&models.Email{}).Where("gmail_account_id = ? AND folder = ?", account.ID, folder)
	db.Count(&total)

	var rows []models.Email
	db.Order("received_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows)

	items := make([]models.EmailListItem, 0, len(rows))
	for _, e := range rows {
		items = append(items, models.EmailListItem{
			ID:             e.ID,
			Folder:         e.Folder,
			FromAddress:    e.FromAddress,
			FromName:       e.FromName,
			ToAddresses:    e.ToAddresses,
			Subject:        e.Subject,
			Snippet:        e.Snippet,
			HasAttachments: e.HasAttachments,
			Attachments:    e.Attachments(),
			IsRead:         e.IsRead,
			ReceivedAt:     e.ReceivedAt,
		})
	}

	return c.JSON(models.EmailListResponse{Success: true, Emails: items, Total: total})
}

// GetEmail - GET /api/v1/emails/:id - helyi metaadat + élő body lekérés
// a Gmailtől (a body sosem kerül tárolásra).
func (h *EmailHandler) GetEmail(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailDetailResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.EmailDetailResponse{Success: false, Message: "Invalid email id"})
	}

	var email models.Email
	if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", id, account.ID).First(&email).Error; err != nil {
		return c.Status(404).JSON(models.EmailDetailResponse{Success: false, Message: "Email not found"})
	}

	full, err := h.gmailAPI.GetFullMessage(c.Context(), account, email.GmailMessageID)
	if err != nil {
		return c.Status(502).JSON(models.EmailDetailResponse{Success: false, Message: "Failed to fetch email body from Gmail: " + err.Error()})
	}

	if !email.IsRead {
		database.GetDB().Model(&email).Update("is_read", true)
	}

	return c.JSON(models.EmailDetailResponse{
		Success:     true,
		ID:          email.ID,
		Folder:      email.Folder,
		Subject:     full.Subject,
		From:        full.FromAddress,
		To:          full.ToAddresses,
		BodyText:    full.BodyText,
		BodyHTML:    full.BodyHTML,
		Attachments: full.Attachments,
		ReceivedAt:  full.ReceivedAt,
	})
}

// GetAttachment - GET /api/v1/emails/:id/attachments/:attachmentId - proxyzott
// letöltés: a Gmail token sosem jut el a böngészőhöz.
func (h *EmailHandler) GetAttachment(c *fiber.Ctx) error {
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

	attachmentID := c.Params("attachmentId")
	var filename string
	for _, a := range email.Attachments() {
		if a.AttachmentID == attachmentID {
			filename = a.Filename
			break
		}
	}
	if filename == "" {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Attachment not found on this email"})
	}

	data, err := h.gmailAPI.GetAttachment(c.Context(), account, email.GmailMessageID, attachmentID)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to fetch attachment from Gmail: " + err.Error()})
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	return c.Send(data)
}

// SendEmail - POST /api/v1/emails/send - új levél vagy válasz küldése
func (h *EmailHandler) SendEmail(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	account, err := currentGmailAccount(userID)
	if err != nil {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "Connect your Gmail account first"})
	}

	var req models.EmailSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "Invalid request body"})
	}
	if req.To == "" || req.Subject == "" {
		return c.Status(400).JSON(models.EmailSendResponse{Success: false, Message: "to and subject are required"})
	}

	var inReplyToHeader, referencesHeader string
	if req.InReplyToEmailID != 0 {
		var original models.Email
		if err := database.GetDB().Where("id = ? AND gmail_account_id = ?", req.InReplyToEmailID, account.ID).First(&original).Error; err == nil {
			full, err := h.gmailAPI.GetFullMessage(c.Context(), account, original.GmailMessageID)
			if err == nil {
				inReplyToHeader = full.MessageIDHeader
				referencesHeader = full.ReferencesHeader
			}
		}
	}

	raw := services.BuildRawMessage(account.EmailAddress, req.To, req.Subject, req.Body, inReplyToHeader, referencesHeader)
	gmailMessageID, err := h.gmailAPI.SendMessage(c.Context(), account, raw)
	if err != nil {
		return c.Status(502).JSON(models.EmailSendResponse{Success: false, Message: "Failed to send email: " + err.Error()})
	}

	return c.JSON(models.EmailSendResponse{Success: true, GmailMessageID: gmailMessageID})
}
```

- [ ] **Step 2: Write the routes**

```go
// backend/internal/routes/email_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupEmailRoutes(api fiber.Router) {
	h := handlers.NewEmailHandler()
	emails := api.Group("/emails")
	emails.Use(middleware.JWTMiddleware())
	emails.Use(middleware.RequirePermission("gmail.manage"))

	emails.Get("/", h.ListEmails)
	emails.Get("/:id", h.GetEmail)
	emails.Get("/:id/attachments/:attachmentId", h.GetAttachment)
	emails.Post("/send", h.SendEmail)
}
```

- [ ] **Step 3: Register the route group**

In `backend/internal/routes/routes.go`, add after `SetupGmailRoutes(v1)`:

```go
	SetupGmailRoutes(v1)             // Gmail OAuth connect/status endpoints
	SetupEmailRoutes(v1)             // Synced email list/detail/attachment/send endpoints
```

- [ ] **Step 4: Build to verify**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: both exit 0.

- [ ] **Step 5: Manual smoke test**

```bash
docker-compose restart backend
curl -s "http://localhost:8080/api/v1/emails?folder=inbox" -H "Authorization: Bearer <a real JWT>"
```
Expected (no Gmail account connected yet): `{"success":false,"message":"Connect your Gmail account first"}`.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/email_handler.go backend/internal/routes/email_routes.go backend/internal/routes/routes.go
git commit -m "feat(gmail): add email list/detail/attachment/send endpoints"
```

---

### Task 11: Invoice-notice handler (send + list) + routes

**Files:**
- Create: `backend/internal/handlers/invoice_notice_handler.go`
- Modify: `backend/internal/routes/invoice_routes.go`

**Interfaces:**
- Consumes: `checkInvoiceAccess` (Task 9); `models.InvoiceNotice`, `models.InvoiceNoticeSendRequest`, `models.InvoiceNoticeResponse`, `models.InvoiceNoticeListResponse` (Task 2); `models.ProjectClient`, `models.Client`, `models.Project`, `models.GmailAccount` (existing/Task 2); `services.BuildRawMessage`, `services.NewRealGmailAPI` (Task 6).
- Produces: routes `POST /api/v1/projects/:id/invoice-notice`, `GET /api/v1/projects/:id/invoice-notices` — consumed by Task 13 (invoice page's "Értesítő küldése" button).

- [ ] **Step 1: Write the handler**

```go
// backend/internal/handlers/invoice_notice_handler.go
package handlers

import (
	"fmt"
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type InvoiceNoticeHandler struct {
	permissionService *services.PermissionService
}

func NewInvoiceNoticeHandler() *InvoiceNoticeHandler {
	return &InvoiceNoticeHandler{permissionService: services.NewPermissionService()}
}

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

	subject := fmt.Sprintf("Számla értesítő - %s", project.Name)
	body := fmt.Sprintf(
		"Kedves %s!\n\nÉrtesítjük, hogy hamarosan számlát állítunk ki a(z) \"%s\" projekt kapcsán.\n\nÜdvözlettel",
		client.Name, project.Name,
	)

	raw := services.BuildRawMessage(account.EmailAddress, client.Email, subject, body, "", "")
	gmailMessageID, err := services.NewRealGmailAPI().SendMessage(c.Context(), &account, raw)
	if err != nil {
		return c.Status(502).JSON(models.InvoiceNoticeResponse{Success: false, Message: "Failed to send notice email: " + err.Error()})
	}

	notice := models.InvoiceNotice{
		ProjectID:      uint(projectID),
		ClientID:       req.ClientID,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		GmailMessageID: gmailMessageID,
		SentBy:         currentUserID,
		SentAt:         time.Now(),
	}
	db.Create(&notice)

	return c.JSON(models.InvoiceNoticeResponse{Success: true, Notice: &notice})
}

// ListInvoiceNotices - GET /api/v1/projects/:id/invoice-notices?period_start=&period_end=
func (h *InvoiceNoticeHandler) ListInvoiceNotices(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(models.InvoiceNoticeListResponse{Success: false, Message: "Invalid project id"})
	}

	query := database.GetDB().Where("project_id = ?", projectID)
	if ps := c.Query("period_start"); ps != "" {
		query = query.Where("period_start = ?", ps)
	}
	if pe := c.Query("period_end"); pe != "" {
		query = query.Where("period_end = ?", pe)
	}

	var notices []models.InvoiceNotice
	query.Order("sent_at desc").Find(&notices)

	return c.JSON(models.InvoiceNoticeListResponse{Success: true, Notices: notices})
}
```

- [ ] **Step 2: Register the routes**

Open `backend/internal/routes/invoice_routes.go`, find the existing `api.Group("/projects")` block (the one carrying `.Use(middleware.JWTMiddleware())` and the `POST /:id/invoices` registration), and add, next to the other project-scoped invoice routes:

```go
	noticeHandler := handlers.NewInvoiceNoticeHandler()
	// POST /api/v1/projects/:id/invoice-notice - Számla-értesítő e-mail küldése az ügyfélnek
	projects.Post("/:id/invoice-notice", noticeHandler.SendInvoiceNotice)
	// GET /api/v1/projects/:id/invoice-notices - Elküldött értesítők listázása
	projects.Get("/:id/invoice-notices", noticeHandler.ListInvoiceNotices)
```

(`projects` here is the existing `api.Group("/projects")` variable already in that file — add these two lines alongside the existing `projects.Post("/:id/invoices", ...)` registration, not a new group.)

- [ ] **Step 3: Build to verify**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go vet ./...`
Expected: both exit 0.

- [ ] **Step 4: Manual smoke test**

```bash
docker-compose restart backend
curl -s -X POST http://localhost:8080/api/v1/projects/1/invoice-notice \
  -H "Authorization: Bearer <a real JWT>" -H "Content-Type: application/json" \
  -d '{"client_id": 1}'
```
Expected (no Gmail account connected yet): `{"success":false,"message":"Connect your Gmail account first"}`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/invoice_notice_handler.go backend/internal/routes/invoice_routes.go
git commit -m "feat(gmail): add invoice-notice send/list endpoints"
```

---

### Task 12: Frontend services — `gmailService.ts`, `emailsService.ts`

**Files:**
- Create: `frontend/src/services/gmailService.ts`
- Create: `frontend/src/services/emailsService.ts`

**Interfaces:**
- Consumes: `apiClient` (`frontend/src/lib/api.ts`); backend response shapes from Task 8/10 (`GmailStatusResponse`, `EmailListResponse`, `EmailDetailResponse`, `EmailSendResponse`).
- Produces: `GmailService.getAuthURL()`, `GmailService.getStatus()`, `GmailService.disconnect()`; `EmailsService.list(folder, page)`, `EmailsService.get(id)`, `EmailsService.getAttachmentUrl(emailId, attachmentId)`, `EmailsService.send(payload)` — consumed by Task 14 (`/dashboard/emails/page.tsx`) and Task 15 (invoice page).

- [ ] **Step 1: Write `gmailService.ts`**

```ts
// frontend/src/services/gmailService.ts
import { apiClient } from '@/lib/api'

export interface GmailStatus {
    success: boolean
    connected: boolean
    email_address?: string
    last_synced_at?: string
    needs_reauth?: boolean
}

export const GmailService = {
    async getAuthURL(): Promise<string> {
        const res = await apiClient.get<{ success: boolean; url: string }>('/gmail/auth-url')
        return res.url
    },

    async getStatus(): Promise<GmailStatus> {
        return apiClient.get<GmailStatus>('/gmail/status')
    },

    async disconnect(): Promise<void> {
        await apiClient.post('/gmail/disconnect')
    },
}
```

- [ ] **Step 2: Write `emailsService.ts`**

```ts
// frontend/src/services/emailsService.ts
import { apiClient } from '@/lib/api'

export interface EmailAttachment {
    filename: string
    size: number
    attachment_id: string
}

export interface EmailListItem {
    id: number
    folder: 'inbox' | 'sent'
    from_address: string
    from_name: string
    to_addresses: string
    subject: string
    snippet: string
    has_attachments: boolean
    attachments: EmailAttachment[]
    is_read: boolean
    received_at: string
}

export interface EmailListResponse {
    success: boolean
    message?: string
    emails?: EmailListItem[]
    total?: number
}

export interface EmailDetail {
    success: boolean
    message?: string
    id?: number
    folder?: string
    subject?: string
    from?: string
    to?: string
    body_text?: string
    body_html?: string
    attachments?: EmailAttachment[]
    received_at?: string
}

export interface SendEmailRequest {
    to: string
    subject: string
    body: string
    in_reply_to_email_id?: number
}

export const EmailsService = {
    async list(folder: 'inbox' | 'sent', page = 1): Promise<EmailListResponse> {
        return apiClient.get<EmailListResponse>('/emails', { folder, page })
    },

    async get(id: number): Promise<EmailDetail> {
        return apiClient.get<EmailDetail>(`/emails/${id}`)
    },

    getAttachmentUrl(emailId: number, attachmentId: string): string {
        return `/emails/${emailId}/attachments/${attachmentId}`
    },

    async downloadAttachment(emailId: number, attachmentId: string, filename: string): Promise<void> {
        const blob = await apiClient.getBlob(EmailsService.getAttachmentUrl(emailId, attachmentId))
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = filename
        a.click()
        window.URL.revokeObjectURL(url)
    },

    async send(payload: SendEmailRequest): Promise<{ success: boolean; message?: string; gmail_message_id?: string }> {
        return apiClient.post('/emails/send', payload)
    },
}
```

- [ ] **Step 3: Type-check**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/gmailService.ts frontend/src/services/emailsService.ts
git commit -m "feat(gmail): add gmailService and emailsService frontend clients"
```

---

### Task 13: "E-mailek" nav link

**Files:**
- Modify: `frontend/src/components/dashboard/DashboardNav.tsx`

**Interfaces:**
- Consumes: `hasAnyPermission` (existing util).

- [ ] **Step 1: Add the link**

In `frontend/src/components/dashboard/DashboardNav.tsx`, add `Mail` to the `lucide-react` import and a new entry to `links`:

```tsx
import { LayoutDashboard, KanbanSquare, Users, Building2, ShieldCheck, Mail } from 'lucide-react'
```

```tsx
        {
            href: '/dashboard/emails',
            label: 'E-mailek',
            icon: Mail,
            show: hasAnyPermission(user, ['gmail.manage']),
        },
```

Place it after the `Clients` entry and before `Team Members`, matching the existing ordering of permission-gated links.

- [ ] **Step 2: Type-check**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Manual check**

With the dev server running, log in as the super_admin user and confirm "E-mailek" appears in the nav bar (it won't navigate anywhere useful until Task 14 ships the page — a 404 at this point is expected and fine for this step).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/dashboard/DashboardNav.tsx
git commit -m "feat(gmail): add Emails nav link gated on gmail.manage"
```

---

### Task 14: `/dashboard/emails` page — connect card, inbox/sent tabs, detail view, compose/reply

**Files:**
- Create: `frontend/src/app/dashboard/emails/page.tsx`

**Interfaces:**
- Consumes: `GmailService` (Task 12), `EmailsService`/`EmailListItem`/`EmailDetail` (Task 12).

- [ ] **Step 1: Write the page**

```tsx
// frontend/src/app/dashboard/emails/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { Mail, Send, Paperclip, RefreshCw, LogOut, X } from 'lucide-react'
import { GmailService, GmailStatus } from '@/services/gmailService'
import { EmailsService, EmailListItem, EmailDetail } from '@/services/emailsService'
import LoadingState from '@/components/ui/LoadingState'

type Folder = 'inbox' | 'sent'

export default function EmailsPage() {
    const searchParams = useSearchParams()
    const [status, setStatus] = useState<GmailStatus | null>(null)
    const [loadingStatus, setLoadingStatus] = useState(true)
    const [connecting, setConnecting] = useState(false)

    const [folder, setFolder] = useState<Folder>('inbox')
    const [emails, setEmails] = useState<EmailListItem[]>([])
    const [loadingEmails, setLoadingEmails] = useState(false)
    const [selected, setSelected] = useState<EmailDetail | null>(null)
    const [selectedId, setSelectedId] = useState<number | null>(null)

    const [composeOpen, setComposeOpen] = useState(false)
    const [composeTo, setComposeTo] = useState('')
    const [composeSubject, setComposeSubject] = useState('')
    const [composeBody, setComposeBody] = useState('')
    const [replyToId, setReplyToId] = useState<number | undefined>(undefined)
    const [sending, setSending] = useState(false)
    const [sendError, setSendError] = useState<string | null>(null)

    useEffect(() => {
        GmailService.getStatus()
            .then(setStatus)
            .finally(() => setLoadingStatus(false))
    }, [])

    useEffect(() => {
        if (!status?.connected) return
        setLoadingEmails(true)
        EmailsService.list(folder)
            .then(res => setEmails(res.emails || []))
            .finally(() => setLoadingEmails(false))
    }, [status?.connected, folder])

    const handleConnect = async () => {
        setConnecting(true)
        const url = await GmailService.getAuthURL()
        window.location.href = url
    }

    const handleDisconnect = async () => {
        await GmailService.disconnect()
        setStatus({ success: true, connected: false })
        setEmails([])
        setSelected(null)
    }

    const openEmail = async (item: EmailListItem) => {
        setSelectedId(item.id)
        const detail = await EmailsService.get(item.id)
        setSelected(detail)
        setEmails(prev => prev.map(e => (e.id === item.id ? { ...e, is_read: true } : e)))
    }

    const openCompose = (reply?: EmailDetail) => {
        setSendError(null)
        if (reply) {
            setComposeTo(reply.from || '')
            setComposeSubject(reply.subject?.startsWith('Re:') ? reply.subject : `Re: ${reply.subject || ''}`)
            setComposeBody('')
            setReplyToId(reply.id)
        } else {
            setComposeTo('')
            setComposeSubject('')
            setComposeBody('')
            setReplyToId(undefined)
        }
        setComposeOpen(true)
    }

    const handleSend = async () => {
        setSendError(null)
        if (!composeTo.trim() || !composeSubject.trim()) {
            setSendError('A címzett és a tárgy megadása kötelező.')
            return
        }
        try {
            setSending(true)
            const res = await EmailsService.send({
                to: composeTo.trim(),
                subject: composeSubject.trim(),
                body: composeBody,
                in_reply_to_email_id: replyToId,
            })
            if (!res.success) {
                setSendError(res.message || 'Küldés sikertelen')
                return
            }
            setComposeOpen(false)
        } catch (err: any) {
            setSendError(err.message)
        } finally {
            setSending(false)
        }
    }

    if (loadingStatus) return <LoadingState message="Gmail állapot betöltése..." />

    if (!status?.connected) {
        return (
            <div className="flex items-center justify-center py-24">
                <div className="bg-card border border-border rounded-2xl shadow-lg p-10 max-w-md text-center space-y-4">
                    <div className="mx-auto w-14 h-14 rounded-full bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center">
                        <Mail className="text-primary" size={28} />
                    </div>
                    <h1 className="text-xl font-semibold text-foreground">Gmail összekapcsolása</h1>
                    <p className="text-sm text-muted-foreground">
                        Kösd össze a Gmail-fiókodat, hogy itt lásd az összes bejövő és elküldött e-mailedet, és
                        közvetlenül innen küldhess számla-értesítőket az ügyfeleknek.
                    </p>
                    {searchParams.get('error') === 'oauth_failed' && (
                        <p className="text-sm text-destructive">Az összekapcsolás sikertelen volt, próbáld újra.</p>
                    )}
                    <button
                        onClick={handleConnect}
                        disabled={connecting}
                        className="w-full px-4 py-2.5 bg-primary text-white rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
                    >
                        {connecting ? 'Átirányítás...' : 'Gmail összekapcsolása'}
                    </button>
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">E-mailek</h1>
                    <p className="text-sm text-muted-foreground">
                        {status.email_address}
                        {status.last_synced_at && ` · Utolsó szinkronizáció: ${new Date(status.last_synced_at).toLocaleString('hu-HU')}`}
                    </p>
                    {status.needs_reauth && (
                        <p className="text-sm text-destructive mt-1">
                            A Gmail hozzáférés lejárt, kösd össze újra a fiókot.
                        </p>
                    )}
                </div>
                <div className="flex gap-2">
                    <button
                        onClick={() => openCompose()}
                        className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg font-medium hover:bg-primary/90 transition-colors"
                    >
                        <Send size={16} /> Új levél
                    </button>
                    <button
                        onClick={handleDisconnect}
                        className="flex items-center gap-2 px-3 py-2 text-muted-foreground hover:text-destructive rounded-lg transition-colors"
                        title="Gmail leválasztása"
                    >
                        <LogOut size={16} />
                    </button>
                </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-5 gap-6">
                <div className="md:col-span-2 bg-card border border-border rounded-xl shadow-sm overflow-hidden">
                    <div className="flex border-b border-border">
                        {(['inbox', 'sent'] as Folder[]).map(f => (
                            <button
                                key={f}
                                onClick={() => { setFolder(f); setSelected(null); setSelectedId(null) }}
                                className={`flex-1 py-3 text-sm font-medium transition-colors ${
                                    folder === f
                                        ? 'text-primary border-b-2 border-primary bg-primary/5'
                                        : 'text-muted-foreground hover:text-foreground'
                                }`}
                            >
                                {f === 'inbox' ? 'Beérkezett' : 'Elküldött'}
                            </button>
                        ))}
                    </div>

                    {loadingEmails ? (
                        <div className="p-6"><LoadingState message="E-mailek betöltése..." /></div>
                    ) : emails.length === 0 ? (
                        <p className="p-6 text-sm text-muted-foreground text-center">Nincs megjeleníthető e-mail.</p>
                    ) : (
                        <div className="divide-y divide-border max-h-[70vh] overflow-y-auto">
                            {emails.map(item => (
                                <button
                                    key={item.id}
                                    onClick={() => openEmail(item)}
                                    className={`w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors ${
                                        selectedId === item.id ? 'bg-primary/5' : ''
                                    }`}
                                >
                                    <div className="flex justify-between items-baseline gap-2">
                                        <span className={`text-sm truncate ${!item.is_read ? 'font-semibold text-foreground' : 'text-muted-foreground'}`}>
                                            {folder === 'inbox' ? (item.from_name || item.from_address) : item.to_addresses}
                                        </span>
                                        <span className="text-xs text-muted-foreground shrink-0">
                                            {new Date(item.received_at).toLocaleDateString('hu-HU')}
                                        </span>
                                    </div>
                                    <div className={`text-sm truncate ${!item.is_read ? 'font-medium text-foreground' : 'text-muted-foreground'}`}>
                                        {item.subject || '(nincs tárgy)'}
                                        {item.has_attachments && <Paperclip size={12} className="inline ml-1 align-text-top" />}
                                    </div>
                                    <div className="text-xs text-muted-foreground truncate">{item.snippet}</div>
                                </button>
                            ))}
                        </div>
                    )}
                </div>

                <div className="md:col-span-3 bg-card border border-border rounded-xl shadow-sm p-6 min-h-[300px]">
                    {!selected ? (
                        <div className="h-full flex items-center justify-center text-muted-foreground text-sm">
                            Válassz egy e-mailt a bal oldali listából.
                        </div>
                    ) : (
                        <div className="space-y-4">
                            <div>
                                <h2 className="text-lg font-semibold text-foreground">{selected.subject || '(nincs tárgy)'}</h2>
                                <p className="text-sm text-muted-foreground">Feladó: {selected.from}</p>
                                <p className="text-sm text-muted-foreground">Címzett: {selected.to}</p>
                            </div>
                            <div className="prose prose-sm max-w-none text-foreground whitespace-pre-wrap">
                                {selected.body_text || '(üres törzs)'}
                            </div>
                            {selected.attachments && selected.attachments.length > 0 && (
                                <div className="border-t border-border pt-3 space-y-1">
                                    <p className="text-xs font-medium text-muted-foreground">Csatolmányok</p>
                                    {selected.attachments.map(a => (
                                        <button
                                            key={a.attachment_id}
                                            onClick={() => selected.id && EmailsService.downloadAttachment(selected.id, a.attachment_id, a.filename)}
                                            className="flex items-center gap-2 text-sm text-primary hover:underline"
                                        >
                                            <Paperclip size={14} /> {a.filename}
                                        </button>
                                    ))}
                                </div>
                            )}
                            <button
                                onClick={() => openCompose(selected)}
                                className="flex items-center gap-2 px-4 py-2 bg-muted text-foreground rounded-lg text-sm font-medium hover:bg-muted/70 transition-colors"
                            >
                                <RefreshCw size={14} /> Válasz
                            </button>
                        </div>
                    )}
                </div>
            </div>

            {composeOpen && (
                <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50">
                    <div className="bg-card rounded-xl shadow-xl p-6 w-full max-w-lg space-y-4">
                        <div className="flex justify-between items-center">
                            <h3 className="text-lg font-semibold">{replyToId ? 'Válasz' : 'Új levél'}</h3>
                            <button onClick={() => setComposeOpen(false)} className="text-muted-foreground hover:text-foreground">
                                <X size={18} />
                            </button>
                        </div>
                        {sendError && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {sendError}
                            </div>
                        )}
                        <input
                            type="email"
                            placeholder="Címzett"
                            value={composeTo}
                            onChange={e => setComposeTo(e.target.value)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <input
                            type="text"
                            placeholder="Tárgy"
                            value={composeSubject}
                            onChange={e => setComposeSubject(e.target.value)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <textarea
                            placeholder="Üzenet"
                            rows={6}
                            value={composeBody}
                            onChange={e => setComposeBody(e.target.value)}
                            className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        />
                        <div className="flex justify-end gap-3">
                            <button onClick={() => setComposeOpen(false)} className="px-4 py-2 text-foreground bg-muted rounded-lg hover:bg-muted/70">
                                Mégse
                            </button>
                            <button
                                onClick={handleSend}
                                disabled={sending}
                                className="px-4 py-2 bg-primary text-white rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50"
                            >
                                {sending ? 'Küldés...' : 'Küldés'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
```

- [ ] **Step 2: Type-check**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Manual browser test**

With the dev stack running and real `GOOGLE_OAUTH_CLIENT_ID`/`SECRET`/`REDIRECT_URL` set (the user provides these), navigate to `/dashboard/emails`, click "Gmail összekapcsolása", complete Google's consent screen, confirm the redirect lands back on `/dashboard/emails?connected=1` while still logged in, and that Inbox/Sent populate after the first sync run. Send a test email only to an address the user provides (never a real client).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/dashboard/emails/page.tsx
git commit -m "feat(gmail): add Emails page with connect flow, inbox/sent, detail, compose/reply"
```

---

### Task 15: Invoice page — "Értesítő küldése" button + status

**Files:**
- Modify: `frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx`

**Interfaces:**
- Consumes: a new `sendInvoiceNotice(projectId, payload)` function added to `frontend/src/services/invoicesService.ts` in Step 1 of this task; the page's existing `project`, `selectedClientId` (or equivalent selected-client state), `periodStart`/`periodEnd` state already present on this page from prior work.

- [ ] **Step 1: Add the service call**

Add to `frontend/src/services/invoicesService.ts` (alongside the file's other `ProjectsService`-style exported functions):

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
}
```

- [ ] **Step 2: Read the invoice page's current client/period state**

Run: `grep -n "selectedClientId\|clientId\|periodStart\|periodEnd\|useState" frontend/src/app/dashboard/board/\[projectId\]/invoice/page.tsx` to confirm the exact existing state variable names for the selected client and the invoicing period before wiring the new button — reuse those names rather than introducing parallel ones.

- [ ] **Step 3: Add notice state, effect, and handler**

Add near the page's other `useState` calls:

```tsx
const [noticeSending, setNoticeSending] = useState(false)
const [noticeSentAt, setNoticeSentAt] = useState<string | null>(null)
const [noticeError, setNoticeError] = useState<string | null>(null)
```

Add a handler (adapt `selectedClientId`/`periodStart`/`periodEnd` to whatever this page's existing variable names are, per Step 2):

```tsx
const handleSendInvoiceNotice = async () => {
    if (!project || !selectedClientId) return
    setNoticeError(null)
    try {
        setNoticeSending(true)
        const res = await InvoiceNoticesService.send(project.id, {
            client_id: selectedClientId,
            period_start: periodStart || undefined,
            period_end: periodEnd || undefined,
        })
        if (!res.success) {
            setNoticeError(res.message || 'Az értesítő küldése sikertelen')
            return
        }
        setNoticeSentAt(res.notice?.sent_at || new Date().toISOString())
    } catch (err: any) {
        setNoticeError(err.message)
    } finally {
        setNoticeSending(false)
    }
}
```

Add the import: `import { InvoiceNoticesService } from '@/services/invoicesService'`.

- [ ] **Step 4: Add the button and status to the JSX**

Add this block near the page's existing "issue invoice" action button (same section, so both actions sit together):

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

- [ ] **Step 5: Type-check**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 6: Manual browser test**

Open a project's invoice page, connect Gmail (Task 14) if not already connected, pick a client with an email address on file, click "Értesítő küldése", confirm the "Elküldve: ..." status appears and a real email only reaches an address the user explicitly provided for testing.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/services/invoicesService.ts "frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx"
git commit -m "feat(gmail): add invoice-notice send button to the invoice page"
```

---

## Self-Review

**Spec coverage:**
- OAuth connect/disconnect/status: Task 8. ✓
- 3-hourly History API sync + 30-day initial backfill + history-expired fallback: Task 7. ✓
- Metadata-only local mirror (no body/attachment storage): Task 2 (`Email` has no body field) + Task 10 (`GetEmail`/`GetAttachment` always call Gmail live). ✓
- Emails page (Inbox/Sent, detail, compose/reply, "extra" UI): Task 14. ✓
- Invoice-notice send + status, independent of invoice issuance: Task 11 (backend) + Task 15 (frontend). ✓
- `gmail.manage` permission, super_admin only, gating all Gmail/email routes: Task 1 (migration) + Tasks 8/10 (`RequirePermission("gmail.manage")`) + Task 13 (nav gating). ✓
- New env vars documented as the user's responsibility: Global Constraints + Task 5. ✓
- Plaintext token storage matching `billingo_settings.api_key` precedent: Task 2 model (no encryption), stated explicitly in Global Constraints. ✓
- Tokens never reach the browser (attachment proxy): Task 10 `GetAttachment`. ✓
- Reuse of `invoices.create` permission + `project_clients` attachment check for invoice-notice: Task 11 (mirrors `invoice_handler.go`'s exact check). ✓

**Placeholder scan:** No TBD/TODO markers; every step has literal code or an exact shell command. Task 15 Step 2 intentionally directs the implementer to `grep` the current file for exact existing variable names rather than guessing them, since this plan was written without re-reading that file's full current state in this session — this is a discovery step with a concrete command, not a vague placeholder.

**Type/signature consistency:**
- `GmailAPI.GetAttachment` returns `([]byte, error)` consistently in the interface (Task 4), `RealGmailAPI` (Task 6), and its caller in `email_handler.go` (Task 10) — corrected from an earlier draft that mismatched a `(filename, contentType)` return Gmail's attachments.get endpoint doesn't actually provide.
- `BuildRawMessage(fromAddress, to, subject, body, inReplyToHeader, referencesHeader string) []byte` signature matches between its definition (Task 6) and both call sites (Task 10 `SendEmail`, Task 11 `SendInvoiceNotice`).
- `checkInvoiceAccess(ps *services.PermissionService, userID uint, action string) error` signature matches between its Task 9 redefinition and its Task 11 call site.
- `models.Email.Attachments()` / `models.AttachmentMetaToJSON` names match between Task 2's definition and Task 7/10's usage.
- `EmailSendRequest.InReplyToEmailID uint` (a local `emails.id`, not a Gmail message id) matches between Task 2's model and Task 10's handler logic that resolves it to a `GmailMessageIDHeader` before sending.
