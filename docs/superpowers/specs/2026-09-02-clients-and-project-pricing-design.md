# Clients CRUD, Project Pricing, Board Create-Project — Design

Date: 2026-09-02
Status: Approved by user, implementing.

## Goal

1. Add a "create new project" entry point on the board project-picker (`/dashboard/board`).
2. Add a "Clients" (Ügyfelek) nav section with full CRUD, storing data required to issue a
   Hungarian-compliant invoice.
3. Add pricing-agreement fields (hourly vs fixed) to **Project**, not Client.
4. Allow a Project to be linked to one or more Clients.

## Data model

### `clients` table (new)
- `id`, `type` (`company`|`individual`), `name`
- `tax_number` (adószám) — required if `type='company'`, optional for `individual`
- `eu_vat_number` (optional), `company_reg_number` (optional, company only)
- `billing_zip`, `billing_city`, `billing_address`
- `bank_account_number`, `email`, `phone`, `notes` (all optional)
- `is_active`, `created_by`, `created_at`, `updated_at`

### `project_clients` join table (new)
`id`, `project_id` FK → projects, `client_id` FK → clients, `assigned_at`, `assigned_by`,
`UNIQUE(project_id, client_id)`. Mirrors `project_assignments`.

### `projects` table — new columns
`pricing_type` (`hourly`|`fixed`, nullable), `hourly_rate` (numeric, HUF, nullable),
`fixed_price` (numeric, HUF, nullable).

## Backend (mirrors existing Project CRUD conventions exactly)

- `internal/models/client.go`, `internal/models/project_client.go` — GORM structs +
  Create/Update/Response/ListResponse DTOs, same shape as `project.go` / `project_assignment.go`.
- `internal/handlers/client_handler.go` — CRUD. Mutations require `clients.create/update/delete`
  permission, falling back to role `admin`/`super_admin` (identical pattern to `project_handler.go`).
  List/read open to any authenticated user.
- `internal/handlers/project_client_handler.go` — attach/detach clients on a project. Reuses the
  `checkProjectAccess` pattern from `project_assignment_handler.go` (admin/super_admin/manager).
- `internal/routes/client_routes.go`, `internal/routes/project_client_routes.go`, registered in
  `routes.go` alongside the existing route groups.
- Migrations (plain SQL, matching `000006`/`000007` style, no `.down.sql` per existing convention):
  - `000008_create_clients_table.up.sql` — table + `clients.*` permissions + role grants
    (super_admin/admin: full; manager/user: read+list, matching the `projects.*` grant pattern).
  - `000009_create_project_clients_table.up.sql` — join table + `project_clients.*` permissions +
    role grants (mirrors `project_assignments.*` grants).
  - `000010_add_pricing_to_projects_table.up.sql` — `ALTER TABLE projects ADD COLUMN ...` for the
    3 pricing columns.
- `Project` model gets `PricingType`, `HourlyRate`, `FixedPrice` fields and a
  `Clients []Client gorm:"many2many:project_clients;"` relation; DTOs updated to match.

## Frontend

- `services/clientsService.ts` (mirrors `projectsService.ts`).
- New "Ügyfelek" nav entry in `DashboardNav.tsx` → `/dashboard/clients` page: table (styling mirrors
  `UsersTable.tsx`) + Create/Edit modals (mirror `CreateProjectModal.tsx`/`EditProjectModal.tsx`,
  form fields conditional on `type`).
- `CreateProjectModal.tsx` / `EditProjectModal.tsx` get a pricing section (radio hourly/fixed → rate
  or fixed-price field) and a client multi-select.
- `/dashboard/board` page gets a "New project" button opening the existing `CreateProjectModal`.

## Verification

Backend: migrations auto-run on `devbridge_backend` container restart (existing `RunMigrations`
mechanism); checked via `docker compose logs` and API smoke calls inside the container.
Frontend: verified in-browser against the running `devbridge_frontend` dev server — Clients CRUD,
project pricing form, board create-project flow, permission-gated buttons for a non-admin user.
