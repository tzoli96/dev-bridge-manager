# Email Marketing Contacts — Design

Date: 2026-09-29
Status: Approved by user, ready for implementation planning.

## Goal

Let any logged-in user maintain a single, global list of email-marketing
contacts (name, subscription status, tags, notes) independent of the
existing `Client` records, and move that list in and out of the app via
CSV so it can be synced with an external tool like Mailchimp.

## Context and non-goals

**Confirmed with the user during brainstorming:**
- Fully independent of `Client` — no foreign key, no auto-population from
  clients. A separate, purpose-built list.
- Fields: email (required, unique), first name, last name, subscribed
  (bool), tags (free text), source, notes.
- Tags are a free-text, comma-separated field on the contact row — no
  separate tags table or tag-management UI.
- Import and export both use CSV, the same column layout in both
  directions.
- Import lets the user choose, per upload, whether a row whose email
  already exists is skipped (default) or overwrites the existing row.
- Export can be filtered by subscription status and/or tag; with no
  filter it exports everything.
- Visible and usable by every logged-in user — no permission gate, the
  same "anyone logged in" rule already used by `/activity-digest` and
  `/search`.

**Non-goals (explicitly out of scope for this spec):**
- No relationship to `Client`, projects, or any other existing entity.
- No tag management UI (create/rename/delete tags as first-class
  objects) — tags are just a string on the contact.
- No actual email sending — this is list storage and CSV portability
  only, not a mailer.
- No background job/queue for import or export — both are handled
  synchronously within a single HTTP request (see "Import/export
  mechanism").
- No audit log of who imported/edited/deleted a contact.
- No per-contact history of subscription-status changes.

## Existing foundation (verified, reused as-is)

- Fiber route group pattern with `middleware.JWTMiddleware()` and no
  further permission check, exactly like
  `backend/internal/routes/activity_digest_routes.go`'s
  `/activity-digest` route — the new routes follow this shape rather
  than the `clients.*` RBAC-gated pattern, since the user asked for
  "everyone."
- Multipart file upload handling via Fiber's `c.FormFile`, already used
  in `backend/internal/handlers/attachment_handler.go` — reused for the
  CSV import endpoint instead of introducing a new upload mechanism.
- Existing handler/route/model trio pattern for a small top-level
  resource (e.g. `client.go` + `client_handler.go` + implicit routes
  file) — the new feature follows the same three-file backend shape.
- Existing frontend UI kit components (`components/ui/`): `Button`,
  `Modal`, `Input`, `EmptyState`, `LoadingState`, `ErrorState` — no new
  UI library or component primitives needed.
- `apiClient` (`frontend/src/lib/api.ts`) — reused as-is for the new
  `marketingContactsService.ts`.
- `DashboardNav.tsx`'s `links` array — the new nav entry follows the
  existing `{ href, label, icon, show }` shape, with `show: true` (no
  permission check), matching the `Dashboard`/`Projects` entries.

## Data model

### New table: `marketing_contacts`

- `id` (PK)
- `email` (string, not null, unique index)
- `first_name` (string, nullable)
- `last_name` (string, nullable)
- `subscribed` (bool, not null, default `true`)
- `tags` (string, nullable — free-text, comma-separated, e.g.
  `"hírlevél, vip"`)
- `source` (string, nullable — e.g. `"import"`, `"manuális"`)
- `notes` (text, nullable)
- `created_by` (FK → users, not null)
- `created_at`, `updated_at`

No project/client foreign key (explicitly independent), no separate tags
table (explicitly declined), no history/versioning table (no audit
requirement).

Both `.up.sql` and `.down.sql` migrations, numbered `000042` (next
available number as of this spec — verify at implementation time, since
`000040`/`000041` are recent additions from concurrent work).

## Backend

### `backend/internal/models/marketing_contact.go` (new)

```go
package models

import "time"

type MarketingContact struct {
    ID         uint      `json:"id" gorm:"primaryKey"`
    Email      string    `json:"email" gorm:"not null;uniqueIndex" validate:"required,email"`
    FirstName  string    `json:"first_name"`
    LastName   string    `json:"last_name"`
    Subscribed bool      `json:"subscribed" gorm:"not null;default:true"`
    Tags       string    `json:"tags"`
    Source     string    `json:"source"`
    Notes      string    `json:"notes" gorm:"type:text"`
    CreatedBy  uint      `json:"created_by" gorm:"not null"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

type MarketingContactCreateRequest struct {
    Email      string `json:"email" validate:"required,email"`
    FirstName  string `json:"first_name"`
    LastName   string `json:"last_name"`
    Subscribed *bool  `json:"subscribed"`
    Tags       string `json:"tags"`
    Source     string `json:"source"`
    Notes      string `json:"notes"`
}

type MarketingContactUpdateRequest struct {
    Email      string `json:"email" validate:"required,email"`
    FirstName  string `json:"first_name"`
    LastName   string `json:"last_name"`
    Subscribed *bool  `json:"subscribed"`
    Tags       string `json:"tags"`
    Source     string `json:"source"`
    Notes      string `json:"notes"`
}

// ImportResult is the response body for POST /marketing-contacts/import.
type ImportResult struct {
    Created int      `json:"created"`
    Updated int      `json:"updated"`
    Skipped int      `json:"skipped"`
    Errors  []string `json:"errors"` // e.g. "row 14: invalid email format"
}
```

`Subscribed` is a `*bool` on the request types so "not sent" (defaults to
`true` on create) is distinguishable from "explicitly `false`" on update.

### `backend/internal/handlers/marketing_contact_handler.go` (new)

Standard CRUD plus import/export, mirroring `client_handler.go`'s shape:

- `ListMarketingContacts` — `GET /api/v1/marketing-contacts` — query
  params `search` (matches email/first_name/last_name via `ILIKE`),
  `subscribed` (`true`/`false`), `tag` (matches via `ILIKE '%tag%'` on
  the `tags` column). No pagination for v1 (mailing lists in this app's
  scale are expected to be small; add pagination later if it becomes a
  problem — YAGNI).
- `CreateMarketingContact` — `POST /api/v1/marketing-contacts` — 409 if
  `email` already exists (`ErrDuplicatedKey` from the unique index,
  translated to a friendly message, mirroring how other handlers in this
  codebase surface unique-constraint violations).
- `UpdateMarketingContact` — `PUT /api/v1/marketing-contacts/:id`.
- `DeleteMarketingContact` — `DELETE /api/v1/marketing-contacts/:id` —
  hard delete.
- `ImportMarketingContacts` — `POST /api/v1/marketing-contacts/import` —
  see "Import" below.
- `ExportMarketingContacts` — `GET /api/v1/marketing-contacts/export` —
  see "Export" below.

### Import

- `multipart/form-data` with fields `file` (the CSV) and `overwrite`
  (`"true"`/`"false"`, default `"false"` if absent).
- Reject before parsing if the uploaded file exceeds **5 MB** (`413`) —
  guards the synchronous request against an oversized upload, per the
  "no background job" decision.
- Parse with Go's standard `encoding/csv`, header row required. Expected
  columns: `email,first_name,last_name,subscribed,tags,source,notes` —
  matched by header name (case-insensitive), not position, so column
  order doesn't matter and unknown extra columns are ignored. If the
  header is missing an `email` column, return `400` immediately without
  processing any rows.
- Cap at **20,000 data rows**; a file with more rows is rejected with
  `400` before any row is written (state a hard, simple limit rather
  than silently truncating).
- Per row: validate `email` is non-empty and passes a basic format check
  (same `validate:"email"` tag as `MarketingContactCreateRequest`, used
  standalone via the validator). Invalid rows are skipped, and the 1-based
  row number plus reason is appended to `ImportResult.Errors`; processing
  continues.
- Per valid row, look up by `email`:
  - Not found → insert, increment `Created`.
  - Found, `overwrite=false` → increment `Skipped`, no write.
  - Found, `overwrite=true` → update all fields except `id`/`created_by`,
    increment `Updated`.
- `subscribed` column parses `"true"/"1"/"yes"` (case-insensitive) as
  `true`, everything else (including empty) as `false` — mirrors a
  typical CSV export's boolean encoding without requiring a specific
  casing.
- The whole import runs inside one DB transaction per row (not one
  transaction for the whole file), so a mid-file failure doesn't roll
  back rows already committed — consistent with "processing continues"
  above.
- Returns `200` with the `ImportResult` body even if some rows had
  errors (the errors are reported in the body, not via HTTP status) —
  only structural failures (missing file, missing header, oversized
  file/row count) return a `4xx` for the whole request.

### Export

- `GET /api/v1/marketing-contacts/export?subscribed=true&tag=vip` — same
  filters as `ListMarketingContacts`, both optional.
- Streams a CSV with header `email,first_name,last_name,subscribed,tags,source,notes`
  (using `encoding/csv`), `Content-Type: text/csv`,
  `Content-Disposition: attachment; filename="marketing-contacts.csv"`.
- `subscribed` column written as `true`/`false` literal strings (matches
  what the import parser accepts, so an exported file re-imports
  cleanly).
- No matching rows → `200` with a header-only CSV (not an error).

### Routes (`backend/internal/routes/marketing_contact_routes.go`, new)

```go
func SetupMarketingContactRoutes(api fiber.Router) {
    h := handlers.NewMarketingContactHandler()
    contacts := api.Group("/marketing-contacts")
    contacts.Use(middleware.JWTMiddleware())

    contacts.Get("/", h.ListMarketingContacts)
    contacts.Post("/", h.CreateMarketingContact)
    contacts.Put("/:id", h.UpdateMarketingContact)
    contacts.Delete("/:id", h.DeleteMarketingContact)
    contacts.Post("/import", h.ImportMarketingContacts)
    contacts.Get("/export", h.ExportMarketingContacts)
}
```

No `RequireProjectMember`-style gate and no RBAC permission check — only
a valid JWT, per the "everyone logged in" decision. Registered from
`routes.go` alongside `SetupActivityDigestRoutes(v1)`.

## Frontend

### `frontend/src/services/marketingContactsService.ts` (new)

Thin wrapper over `apiClient`, matching the shape of other services
(e.g. `passwordsService.ts`):
`list(params)`, `create(data)`, `update(id, data)`, `remove(id)`,
`import(file, overwrite)` (builds a `FormData`, posts via `apiClient`'s
underlying fetch — not JSON — since this is a file upload), `exportUrl(params)`
(builds the export URL with query params for a plain `<a href>` /
`window.location` download rather than a fetch, so the browser handles
the file download natively).

### `frontend/src/app/dashboard/marketing-contacts/page.tsx` (new)

- Header: page title "Marketing lista" + "Importálás" button + "Exportálás"
  button + "Új kontakt" button.
- Filter bar: search input (email/name) + subscribed status dropdown
  (mind/feliratkozott/leiratkozott) + tag text filter — all three feed
  into `ListMarketingContacts`'s query params, debounced on the search
  input.
- Body: a table (email, name, subscribed badge, tags, source) with
  edit/delete icon buttons per row — table rather than the password
  page's card grid, since this list is expected to run longer and denser
  than a handful of credentials.
- "Új kontakt" / edit → `Modal` with `Input` fields for email, first
  name, last name, tags, source, notes, and a checkbox for `subscribed`.
- "Importálás" → `Modal` with a file input (`.csv` accept), a "Meglévők
  felülírása" checkbox (unchecked by default), and an "Importálás" submit
  button. On success, shows the `ImportResult` summary inline in the
  modal (created/updated/skipped counts, and the error list if
  non-empty) instead of closing immediately, so the user can read the
  outcome before dismissing.
- "Exportálás" → no modal; directly triggers a download using the
  filter bar's current `subscribed`/`tag` values via
  `marketingContactsService.exportUrl(...)`.
- Empty state: `EmptyState` ("Még nincs marketing kontakt"), with "Új
  kontakt" and "Importálás" actions inline.
- Loading: `LoadingState` while the initial list fetches.

### `frontend/src/components/dashboard/DashboardNav.tsx` (modified)

One new entry in the `links` array, placed after `Clients` and before
`E-mailek` (grouping it with the other contact/relationship-style
entries):

```ts
{
    href: '/dashboard/marketing-contacts',
    label: 'Marketing lista',
    icon: Contact, // lucide-react
    show: true,
},
```

## Security

- No secrets involved — this is plain contact data (email, name), not
  credentials, so no encryption-at-rest requirement (unlike the password
  manager).
- Still requires a valid JWT for every route — not publicly accessible.
- CSV import size/row caps (5 MB, 20,000 rows) prevent an oversized
  synchronous request from tying up a request-handling goroutine for an
  unbounded amount of time.
- No new external dependency: `encoding/csv` and `mime/multipart` are
  both Go standard library, already used elsewhere in this codebase
  (`attachment_handler.go`).

## Testing plan

- Backend unit tests: CSV parsing/import logic — skip-vs-overwrite
  branches, invalid-email row skipped and reported, missing `email`
  header rejected with `400`, oversized file rejected with `413`,
  row-count cap rejected with `400`.
- Handler tests: duplicate email on create returns `409`; update/delete
  of a non-existent `id` returns `404`; list filters (`search`,
  `subscribed`, `tag`) narrow results correctly; export with no matches
  returns a header-only CSV with `200`.
- `go build`/`go vet`/`gofmt` and the frontend's `tsc`/lint, all run in
  this project's documented containers, never on host.
- Manual UI verification in the browser: create/edit/delete a contact,
  import a CSV with a mix of new/duplicate/invalid rows and confirm the
  summary counts, re-import the same file with "overwrite" checked and
  confirm updates land, export with and without filters and confirm the
  downloaded file's contents, empty state.
