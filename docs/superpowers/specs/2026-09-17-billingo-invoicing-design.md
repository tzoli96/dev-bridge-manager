# Billingo Invoicing Integration — Design

Date: 2026-09-17
Status: Approved by user, ready for implementation planning.

## Goal

Allow a project's fixed-price or hourly-billed work to be invoiced directly to
Billingo (Hungarian e-invoicing SaaS) from the project page, using the client
and pricing data that already exist (`Client`, `Project.pricing_type` /
`hourly_rate` / `fixed_price`, `project_clients`, `task_time_entries`).

## Existing foundation (verified, already implemented)

This design builds on top of already-working, uncommitted WIP, confirmed via
`go build`/`go vet` and a live DB inspection during this session:

- `clients` table + `Client` model + full CRUD (`client_handler.go`,
  `client_routes.go`) — a **global** entity, not project-scoped.
- `project_clients` join table — many-to-many between `Project` and `Client`
  (a project can have several clients attached; kept as-is).
- `projects.pricing_type` (`hourly`|`fixed`), `hourly_rate`, `fixed_price`.
- `task_time_entries` (`task_id`, `user_id`, `hours`, `date`) joined to
  `tasks.project_id` — the source of logged hours for hourly billing.

None of this needs to be rebuilt. The Billingo work only adds new pieces on
top of it.

## Scope

1. Add a Billingo partner reference to `Client`.
2. Add a local `Invoice` table recording every invoice created through the
   app (Billingo invoice id, amount, period, status, who created it).
3. Add a `BillingoSettings` table (single row) holding the Billingo API key,
   admin-managed.
4. Add an endpoint to create an invoice for a project: computes the amount,
   calls Billingo, stores the result.
5. Add frontend: "Számla kiállítása" (create invoice) action on the project
   page with client picker + date-range picker (for hourly projects), an
   invoice history list on the project page, and a Billingo settings form
   under the existing "System Settings" admin card.

**Non-goals (explicitly out of scope for this spec):**
- The monthly automatic invoicing job for hourly projects (mentioned by the
  user as a later automation) — this spec only builds the manual,
  on-demand creation flow it will eventually reuse.
- Emailing the invoice to the client — Billingo is told to finalize the
  invoice but not to send it; the team sends it separately themselves.
- Editing/cancelling/storno of an already-created invoice.

## Data model

### `clients` table — new column
- `billingo_partner_id` (string, nullable) — Billingo's partner/customer id
  once this client has been synced to Billingo. Populated lazily: the first
  time an invoice is created for a client, if this is empty, the invoice
  service creates (or looks up) the partner in Billingo and stores the id
  here for reuse on later invoices.

### `invoices` table (new)
- `id`, `project_id` (FK → projects), `client_id` (FK → clients)
- `billingo_invoice_id` (string, Billingo's id for the created invoice)
- `billingo_invoice_number` (string, human-readable invoice number Billingo
  assigns, e.g. `INV-2026-001`)
- `pricing_type` (`hourly`|`fixed`, copied from the project at creation time)
- `period_start`, `period_end` (date, nullable — only set for hourly invoices)
- `amount` (numeric, HUF — the net amount billed)
- `status` (`created`|`failed`) — `failed` rows are kept for audit/debugging
  if the Billingo call errors after we've already validated locally
- `error_message` (text, nullable — populated when `status='failed'`)
- `created_by` (FK → users), `created_at`

### `billingo_settings` table (new, single row)
- `id`, `api_key` (string, plaintext column — matches this project's existing
  convention of storing operational secrets directly in Postgres; no vault/
  secrets-manager exists in this codebase and introducing one is out of
  scope), `block_id` (string, nullable — Billingo requires a "block" id
  identifying which invoice number series/template to use; stored here once
  configured), `updated_by` (FK → users), `updated_at`.
- Table is seeded with exactly one row (id=1) by its migration; the API
  always reads/writes that row. No UI ever creates a second row.

## Backend

### Invoice amount calculation

- **Fixed-price project:** amount = `projects.fixed_price` in full, always.
  A fixed-price project may be invoiced **at most once** — before creating
  the invoice, the handler checks whether an `invoices` row with
  `status='created'` already exists for that `project_id`; if so, the
  request is rejected with 409 ("Ez a projekt már ki lett számlázva").
- **Hourly project:** the user picks a date range (`period_start`,
  `period_end`) in the UI. Amount = `SUM(task_time_entries.hours)` for all
  time entries whose `task.project_id` matches the project, `date BETWEEN
  period_start AND period_end`, multiplied by `projects.hourly_rate`. No
  restriction on repeat invoicing — the same project can be invoiced for
  multiple, presumably non-overlapping, periods over time. Overlap
  detection is out of scope; the user picks the range.

### New endpoint

`POST /api/v1/projects/:id/invoices`

Request body: `{ "client_id": <uint>, "period_start"?: "YYYY-MM-DD",
"period_end"?: "YYYY-MM-DD" }` (`period_start`/`period_end` required when
the project's `pricing_type` is `hourly`, ignored for `fixed`).

Handler flow (`invoice_handler.go`, mirrors `client_handler.go`'s
permission-check pattern):
1. Permission check: `invoices.create`, admin/super_admin/manager fallback
   (same shape as `checkProjectClientAccess`).
2. Load project; reject if it has no `pricing_type` set.
3. Validate `client_id` is actually attached to this project via
   `project_clients` (reject 400 otherwise — you can only bill a client
   that's linked to the project).
4. For `fixed`: check no prior `created` invoice exists for this project
   (409 if so). For `hourly`: validate `period_start <= period_end`.
5. Compute `amount` as above.
6. Resolve `billingo_partner_id` for the client — if empty, call Billingo's
   partner-create endpoint using the client's stored billing fields
   (name, tax number, address, etc.) and persist the returned id.
7. Call Billingo's invoice-create endpoint with the computed amount, the
   partner id, and a finalized status (not draft) — see "Billingo API
   client" below.
8. On success: insert an `invoices` row with `status='created'` and the
   returned Billingo invoice id/number; return it to the caller.
9. On failure calling Billingo: insert an `invoices` row with
   `status='failed'` and the error message, return 502 to the caller with
   the error so the UI can show it.

`GET /api/v1/projects/:id/invoices` — list this project's invoice history
(open to the same users who can view the project), used by the frontend
history list.

### Billingo API client

A small `services/billingo_service.go` wrapping Billingo's REST API behind
two methods: `EnsurePartner(client models.Client) (partnerID string, err
error)` and `CreateInvoice(partnerID string, amount float64, description
string) (invoiceID, invoiceNumber string, err error)`. It reads the API key
and block id from `billingo_settings` on each call (no in-memory caching,
to pick up settings changes without a restart).

The exact Billingo endpoint paths, request/response field names, and
partner/invoice payload shape are **not fixed by this spec** — they must be
confirmed against Billingo's official API documentation during
implementation (this session could not reach their docs to verify current
field names). The service's public method signatures above are the
contract the rest of the app is built against; the HTTP details behind
them are an implementation detail of `billingo_service.go` alone. Both
Billingo calls are made with the invoice finalized (not draft) and with
Billingo's automatic-email option disabled/unset, per the user's explicit
instructions.

VAT (ÁFA) handling is likewise a Billingo-side invoice-item detail, not an
architectural decision for this app: `amount` in the `invoices` table and
in the invoice-create call is the net amount, and `CreateInvoice` applies
Hungary's standard 27% rate unless Billingo's account-level settings for
the resolved partner dictate otherwise (e.g. an EU reverse-charge or
VAT-exempt client) — the exact rate/field is confirmed against Billingo's
API during implementation, same as the other field-name details above.

### Permissions (new migration, following the established convention)

`000016_create_invoices_and_billingo_settings.up.sql`:
- `invoices` and `billingo_settings` tables (as above).
- New permissions: `invoices.create`, `invoices.read`,
  `billingo_settings.manage`.
- Grants: `super_admin`/`admin` get all three; `manager` gets
  `invoices.create`/`invoices.read`; `user` gets `invoices.read` only.
  `billingo_settings.manage` is admin/super_admin only (no manager grant —
  API key management stays admin-restricted).
- `clients.billingo_partner_id` added via `ALTER TABLE clients ADD COLUMN
  billingo_partner_id VARCHAR(100)`.
- No `.down.sql`, matching the convention already used for migrations
  000006–000010 in this codebase.

### Billingo settings endpoints

`GET /api/v1/admin/billingo-settings` / `PUT /api/v1/admin/billingo-settings`
— read/update the single settings row, gated on `billingo_settings.manage`.
The API key is never returned in full by `GET` (masked, e.g. last 4 chars
only) to avoid exposing it to the browser after it's been set once; `PUT`
always requires the full key to change it.

## Frontend

- **Project page:** a "Számla kiállítása" button (visible to users with
  `invoices.create`, same permission-gating pattern as
  `utils/permissions.ts`). Opens a modal: client picker (dropdown of this
  project's attached clients, from the existing `project_clients` data),
  and — only when `project.pricing_type === 'hourly'` — a date-range
  picker for the period. Submitting calls the new endpoint and shows the
  resulting invoice number or the error message on failure.
- **Invoice history:** a list on the project page (below or beside the
  pricing info) showing past invoices for this project — number, client,
  amount, period (if hourly), status, created date — fetched from `GET
  /api/v1/projects/:id/invoices`.
- **Billingo settings:** a new form under the existing "System Settings"
  admin card in `AdminTab.tsx` (replacing its current stub
  `console.log` action) — API key input (masked) and block id input,
  gated on `billingo_settings.manage`.
- New `services/invoicesService.ts` and `services/billingoSettingsService.ts`
  mirroring the existing `clientsService.ts` pattern.

## Error handling

- Billingo API errors (network failure, 4xx/5xx from Billingo, missing/
  invalid API key) are caught in `billingo_service.go`, surfaced to the
  handler as a Go `error`, recorded as a `failed` invoice row, and returned
  to the frontend as a plain-language message so the user knows the
  invoice was **not** created in Billingo and no local state (beyond the
  audit row) was left inconsistent.
- A `failed` row never blocks a subsequent retry — only a `created` row
  counts against the fixed-price once-only rule.

## Testing / Verification plan

- Backend: `docker exec devbridge_backend go build ./... && go vet ./...`
  after each task; targeted handler tests for the amount calculation
  (fixed vs. hourly, the once-only fixed-price rule, hourly date-range
  summation) using the existing test conventions in this codebase.
  Billingo HTTP calls are mocked/stubbed in tests — no real Billingo
  account is touched by automated tests.
- Frontend: `npx tsc --noEmit` inside `devbridge_frontend`; manual
  in-browser verification of the invoice-creation modal (both pricing
  types), the invoice history list, and the admin settings form, including
  permission-gated visibility for a non-admin user.
- Manual, explicitly-approved verification against a real Billingo
  sandbox/test account is needed once before considering this feature
  done, since no automated test can confirm the real API contract — this
  requires a Billingo API key the user provides, entered through the new
  settings UI.
