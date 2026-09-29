# Project Password Manager — Design

Date: 2026-09-29
Status: Approved by user, ready for implementation planning.

## Goal

Let project members store and retrieve shared credentials (server/database
passwords, third-party API keys, etc.) scoped to a single project, visible
and manageable by every member of that project.

## Context and non-goals

**Confirmed with the user during brainstorming:**
- Scoped per project (not a personal, project-independent vault).
- Every active member of a project has equal rights: view, create, edit,
  delete — no view-only vs. manage split (option "A").
- Fields: title, username, password, URL (optional), notes (optional). No
  category/tag field.
- No audit log (no record of who viewed/changed/deleted an entry).
- Lives at `/dashboard/board/[projectId]/passwords`, a new page reached by
  a button from the project overview page — following this codebase's
  existing pattern of project sub-sections as separate Next.js routes
  (`analytics`, `invoice`), not client-side tab state.

**Non-goals (explicitly out of scope for this spec):**
- Audit/history log of views or edits.
- Per-entry granular permissions (owner/manager/member/viewer role
  distinctions within a project's passwords) — every active project member
  has full CRUD.
- Categories/tags/folders for organizing entries.
- Password strength meter, breach checking, or a password generator.
- Sharing an entry across multiple projects, or a global/org-wide vault.
- Rotating the encryption key, or per-entry key versioning — one key for
  the whole app for now (see "Security").

## Existing foundation (verified, reused as-is)

- `ProjectAssignment` (`backend/internal/models/project_assignment.go`)
  already models project membership (`project_id`, `user_id`, `role`,
  `is_active`). Reused as-is for access control — no new membership table.
- `os.Getenv("JWT_SECRET")` with a documented dev fallback
  (`backend/internal/services/auth_service.go`) is the existing pattern
  for app-wide secret material sourced from the environment. The new
  encryption key follows the same shape (`ENCRYPTION_KEY` env var, dev
  fallback, documented as "change in production").
- Project sub-section routing pattern: `analytics/page.tsx` and
  `invoice/page.tsx` under `app/dashboard/board/[projectId]/`, reached via
  a `router.push` button on the project overview page
  (`app/dashboard/board/[projectId]/page.tsx`) — the new `passwords` page
  follows this exact shape.
- Existing frontend UI kit components (`components/ui/`): `Button`,
  `Dialog`/`Modal`, `Input`, `Label`, `Badge`, `EmptyState`, `LoadingState`
  — no new UI library or component primitives needed.
- Existing handler/route/model trio pattern for a small project-scoped
  resource (e.g. `project_client_handler.go` + `project_client_routes.go`)
  — the new feature follows the same three-file backend shape.
- `apiClient` (`frontend/src/lib/api.ts`), the existing fetch wrapper used
  by every frontend service (e.g. `invoicesService.ts`) — reused as-is for
  the new `passwordsService.ts`.

## Important deviation from existing project-visibility behavior

Today, **no** project-scoped resource in this codebase (tasks, boards,
invoices) actually checks `ProjectAssignment` membership — access is
gated only by a user's *global* RBAC permission (e.g. `tasks.create`),
and `GET /api/v1/projects/:id` is explicitly "visible to everyone
logged in." Passwords are more sensitive than task data, so this feature
introduces the **first real enforcement** of `ProjectAssignment` in this
codebase: a user must have an active assignment on the project (any role)
to see or touch its passwords, unless they're `admin`/`super_admin` (who
already bypass project-level checks elsewhere, e.g.
`project_assignment_handler.go`'s `checkProjectAccess`). This is called
out explicitly because it's a new access-control primitive, not a
silently-assumed one.

## Data model

### New table: `project_passwords`

- `id` (PK)
- `project_id` (FK → projects, not null, indexed)
- `title` (string, not null)
- `username` (string, nullable)
- `encrypted_password` (text, not null — AES-GCM ciphertext, base64-encoded;
  never plaintext at rest)
- `url` (string, nullable)
- `notes` (text, nullable)
- `created_by` (FK → users, not null)
- `updated_by` (FK → users, not null)
- `created_at`, `updated_at`

No history/versioning table (no audit log requirement) and no
category/tag column (explicitly declined).

Both `.up.sql` and `.down.sql` migrations, numbered `000041` (next
available number as of this spec).

## Backend

### `backend/internal/services/crypto.go` (new)

```go
package services

func Encrypt(plaintext string) (string, error)
func Decrypt(ciphertext string) (string, error)
```

- Key: `sha256.Sum256([]byte(os.Getenv("ENCRYPTION_KEY")))`, always
  producing a valid 32-byte AES-256 key regardless of the env value's
  length. If `ENCRYPTION_KEY` is unset, falls back to a hardcoded
  development string (mirroring `auth_service.go`'s
  `"default-secret-key-change-in-production"` pattern) — never fails
  startup, but the fallback is clearly named and documented as
  dev-only.
- `Encrypt`: AES-GCM, random 12-byte nonce per call, output is
  `base64(nonce || ciphertext)`.
- `Decrypt`: reverses the above; returns an error if the ciphertext is
  malformed or the auth tag doesn't verify (tampering/wrong key).
- Pure functions, no dependency on `database` or `models` packages, so
  they're unit-testable in isolation (encrypt → decrypt round-trip,
  and a tampered-ciphertext-fails-to-decrypt case).

### `backend/internal/models/project_password.go` (new)

```go
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
// stray `db.Find(&passwords)` handler elsewhere in the codebase can never
// leak it by accident — only buildPasswordDTO's explicit decrypt path can.
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

### `backend/internal/handlers/project_password_handler.go` (new)

Standard CRUD, mirroring `project_client_handler.go`'s shape:

- `GetProjectPasswords` — `GET /api/v1/projects/:id/passwords` — loads
  all `project_passwords` for the project, decrypts each
  `encrypted_password` via `services.Decrypt`, returns
  `[]ProjectPasswordDTO`. A decrypt failure on one row (corrupt data)
  logs and skips that row rather than failing the whole list.
- `CreateProjectPassword` — `POST /api/v1/projects/:id/passwords` —
  encrypts `req.Password` via `services.Encrypt`, sets
  `created_by`/`updated_by` to the current user, inserts, returns the
  created `ProjectPasswordDTO` (201).
- `UpdateProjectPassword` — `PUT /api/v1/projects/:id/passwords/:passwordId`
  — re-encrypts the (possibly changed) password, updates all fields,
  sets `updated_by`, returns the updated `ProjectPasswordDTO`.
- `DeleteProjectPassword` — `DELETE /api/v1/projects/:id/passwords/:passwordId`
  — hard delete (no soft-delete/history requirement).

All four handlers 404 if `passwordId` doesn't belong to `:id`'s project
(scoped lookup: `WHERE id = ? AND project_id = ?`), matching this
codebase's existing pattern for nested resource ownership checks (e.g.
`task_comment_handler.go`'s task-scoped comment lookups).

### `backend/internal/middleware/project_member.go` (new)

```go
func RequireProjectMember() fiber.Handler
```

Reads `userID` from `c.Locals("userID")` (set by `JWTMiddleware`, same as
every other permission middleware) and `:id` from the route params.
Allows the request through if either:
1. The user's role is `admin` or `super_admin` (existing bypass pattern
   from `project_assignment_handler.go`'s `checkProjectAccess`), or
2. An active `ProjectAssignment` row exists for `(project_id = :id,
   user_id = userID, is_active = true)` — any `role` value qualifies,
   consistent with the "A" decision (no per-role distinction).

Otherwise returns 403
`{"success": false, "message": "You are not a member of this project"}`.

### Routes (`backend/internal/routes/project_password_routes.go`, new)

```go
func SetupProjectPasswordRoutes(api fiber.Router) {
    h := handlers.NewProjectPasswordHandler()
    passwords := api.Group("/projects/:id/passwords")
    passwords.Use(middleware.JWTMiddleware())
    passwords.Use(middleware.RequireProjectMember())

    passwords.Get("/", h.GetProjectPasswords)
    passwords.Post("/", h.CreateProjectPassword)
    passwords.Put("/:passwordId", h.UpdateProjectPassword)
    passwords.Delete("/:passwordId", h.DeleteProjectPassword)
}
```

Registered from `main.go` alongside the other `Setup*Routes` calls.

### New environment variables

- `ENCRYPTION_KEY` — added to `backend/.env` (and `.env.example` if one
  exists) with a comment, following the existing "External APIs (later)"
  commented-placeholder style already present in `backend/.env`. A
  missing value falls back to a documented dev-only default (see
  "Security") rather than failing startup, matching `JWT_SECRET`'s
  existing fallback behavior.

## Frontend

### `frontend/src/services/passwordsService.ts` (new)

Thin wrapper over `apiClient`, matching `invoicesService.ts`'s shape:
`listPasswords(projectId)`, `createPassword(projectId, data)`,
`updatePassword(projectId, passwordId, data)`,
`deletePassword(projectId, passwordId)`.

### `frontend/src/app/dashboard/board/[projectId]/passwords/page.tsx` (new)

- Header: back link to the project page (matching `analytics/page.tsx`'s
  and `invoice/page.tsx`'s existing back-link pattern) + page title
  "Jelszavak" + a "+ Új jelszó" `Button` that opens a `Dialog`/`Modal`
  form (title, username, password, URL, notes — all in the existing
  `Input`/`Label` components).
- Body: a responsive card grid (one bordered, rounded-corner panel per
  entry — there is no dedicated `Card` component in this codebase yet, so
  this reuses the plain `div` + Tailwind classes styling already used for
  the invoice list rows in `app/dashboard/board/[projectId]/page.tsx`),
  each showing:
  - Title (bold) and, if set, a small `Badge`-styled host/URL chip.
  - Username with a "copy" icon button (`navigator.clipboard.writeText`).
  - Password rendered as `••••••••` by default; an eye icon toggles
    plaintext reveal client-side (already-fetched, already-decrypted
    value — no extra request on toggle), plus its own "copy" icon
    button.
  - Notes (if set) in muted small text.
  - Edit (pencil icon) and delete (trash icon, with a confirm step —
    reusing whatever confirm pattern this codebase already uses for
    destructive actions, e.g. task/column delete confirmations) in the
    card's corner.
- Empty state: `EmptyState` component ("Még nincs jelszó ehhez a
  projekthez"), with the "+ Új jelszó" action inline.
- Loading: `LoadingState` while the initial list fetches.
- A project member without access (only possible via direct navigation,
  since the button is only rendered for members) sees the 403 rendered
  as an `ErrorState` ("Nincs jogosultságod ehhez a projekthez").

### Project overview page (`app/dashboard/board/[projectId]/page.tsx`, modified)

One new `Button` next to the existing "Analytics" / "Invoice" buttons:
`onClick={() => router.push(\`/dashboard/board/${projectId}/passwords\`)}`,
a lock icon (`lucide-react`'s `Lock` or `KeyRound`, consistent with the
existing icon set already imported in this file).

## Security

- Passwords are encrypted at rest with AES-GCM; the encryption key is
  process-environment-sourced (`ENCRYPTION_KEY`), never stored in the
  database, mirroring `JWT_SECRET`'s existing handling.
- The `ProjectPassword` model's Go struct never JSON-exposes
  `encrypted_password` (`json:"-"`); only `buildPasswordDTO`'s explicit
  `services.Decrypt` call produces the plaintext that reaches the API
  response — and only for authenticated, project-member requests.
- All `/api/v1/projects/:id/passwords*` routes require both a valid JWT
  and active project membership (`RequireProjectMember`), the first
  route in this codebase to enforce `ProjectAssignment` membership
  rather than only a global RBAC permission (see "Important deviation").
- No new secrets-manager, KMS, or external dependency — AES-GCM via Go's
  standard `crypto/aes` and `crypto/cipher` packages only.

## Testing plan

- Backend unit tests: `services.Encrypt`/`Decrypt` round-trip, tampered
  ciphertext fails to decrypt, empty-string edge case.
- Handler tests: a project member gets 200 on all four endpoints; a
  non-member (no active `ProjectAssignment`, not admin/super_admin) gets
  403 from `RequireProjectMember`; a request for a `passwordId` that
  belongs to a different project 404s.
- `go build`/`go vet`/`gofmt` and the frontend's `tsc`/lint, all run in
  this project's documented containers, never on host.
- Manual UI verification in the browser: create/edit/delete a password
  entry, reveal/hide toggle, copy-to-clipboard, empty state, and the
  403 `ErrorState` path (e.g. by temporarily removing a test user's
  `ProjectAssignment` row and confirming the page and API both refuse
  access) — no real external credentials involved, so no gated
  real-account step is needed here (unlike the Jira integration).
