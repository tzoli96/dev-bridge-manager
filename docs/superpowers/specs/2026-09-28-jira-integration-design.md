# Jira Integration for Kanban Boards — Design

Date: 2026-09-28
Status: Approved by user, ready for implementation planning.

## Goal

Let a super admin optionally connect an existing kanban board to a Jira
Cloud project, so that the board additionally shows a read-only mirror of
the Jira issues assigned to a single, fixed Jira account — without
touching or replacing that board's existing local columns/tasks.

## Context and non-goals

This is a one-directional, read-only mirror: Jira is the source of truth
for the mirrored issues' title/description/status. Nothing created or
edited in this app is ever written back to Jira.

**Non-goals (explicitly out of scope for this spec):**
- Two-way sync of any kind (no task edits, moves, or creations in this
  app are pushed to Jira).
- Per-viewing-user Jira identity — a board's Jira connection is tied to
  one fixed Jira account (chosen once, at connect time), and every user
  viewing the board sees that same account's assigned issues, regardless
  of who is logged in.
- Real-time push (Jira webhooks) — a 15-minute poll is sufficient.
- The Jira Agile ("board") API — issues are selected via a JQL search
  against a project key, not via a Jira board id.
- Mirroring issues beyond those assigned to the connected account
  (`assignee = currentUser()`) and not yet in the "Done" status
  category — no full-project mirror.
- Mirroring Jira comments, attachments, or worklogs.
- Encryption of the stored Jira API token beyond this codebase's existing
  posture (see "Security").
- Boards that had a Jira integration and were disconnected keep their
  already-mirrored tasks/columns (frozen, read-only, no longer updated)
  rather than being cleaned up — no "delete on disconnect" path exists.

## Existing foundation (verified, reused as-is)

- `time.Ticker`-based background job pattern
  (`backend/internal/services/scheduler.go`, and its
  `StartProjectRenewalScheduler`/`StartInvoiceReconciliationScheduler`/
  `StartClientStatusEmailScheduler` siblings) — the Jira sync job follows
  the same shape (its own ticker, its own goroutine started from
  `main.go`).
- `billingo_settings` stores an operational secret (API key) as a plain
  column with no encryption layer — the Jira API token follows the same
  existing security posture.
- The `Board` / `KanbanColumn` / `Task` / `TaskPlacement` models in
  `backend/internal/models/kanban.go` are extended in place rather than
  duplicated (see "Data model").
- Admin-only route/dashboard gating pattern already used by
  `SetupProjectRenewalRoutes`, `SetupInvoiceReconciliationRoutes`, etc.
  (`middleware.RequireRole("super_admin")` on the backend,
  `isSuperAdmin(user)` gating the widget/panel on the frontend).
- Small HTTP-client-with-interface pattern used for the AI microservice
  calls (`backend/internal/services/draft_reply.go`,
  `client_status_ai.go`) — the Jira API client follows the same shape
  (an interface for test-double injection, a concrete struct doing the
  real HTTP calls).

## Scope

1. A per-board, opt-in Jira connection: base URL, account email, API
   token, and a Jira project key.
2. A background sync job (every 15 minutes) that pulls the connected
   account's assigned, not-yet-done issues for that project via the Jira
   Issue Search API, and mirrors them into the board as read-only tasks
   in dynamically created, Jira-derived columns.
3. Read-only enforcement on the existing task/column endpoints for
   Jira-sourced tasks/columns — except time tracking and comments, which
   remain fully usable (and stay local only, never sent to Jira).
4. A board-level settings panel (super admin only) to connect/view
   status/disconnect the integration.

## Data model

### New table: `jira_board_integrations`
- `id`, `board_id` (FK → boards, **unique** — at most one integration per
  board)
- `base_url` (string, e.g. `https://yourcompany.atlassian.net`)
- `email` (string — the Jira account's email, used for API Basic Auth)
- `api_token` (string, plaintext column — see "Security")
- `project_key` (string, e.g. `PROJ`)
- `connected_by` (FK → users), `connected_at` (timestamp)
- `last_sync_at` (timestamp, nullable)
- `last_sync_error` (text, nullable — set when the most recent sync run
  for this board failed; cleared on the next successful run)
- `created_at`, `updated_at`

### `Task` (existing table, additive columns)
- `source` (string, default `'local'`; `'jira'` for mirrored issues)
- `jira_issue_key` (string, nullable, e.g. `PROJ-123` — the upsert key
  for sync; unique together with `board_id` in practice, though not
  DB-constrained since a `Task` doesn't carry `board_id` directly today
  and is resolved via its `TaskPlacement`)
- `jira_synced_at` (timestamp, nullable)

### `KanbanColumn` (existing table, additive column)
- `jira_status_name` (string, nullable — set to the Jira `status.name`
  this column mirrors; a non-null value marks the column as
  Jira-derived, auto-created/maintained by the sync job rather than by a
  user)

No new tables for tasks/columns/placements: the sync job upserts directly
into `tasks`, `kanban_columns`, and `task_placements`, exactly like a
human creating/moving a task would, just driven by the scheduler instead
of a request handler. This is the "extend the existing flow" approach
approved during design over a fully parallel `jira_issues`/`jira_columns`
schema — it reuses the existing board-loading, DTO-building, and
time-tracking code paths unchanged.

Both `.up.sql` and `.down.sql` migration files, following this
codebase's numbered-migration convention (next available number at
implementation time).

## Backend

### Connection management endpoints

All under `middleware.RequireRole("super_admin")`, mirroring
`SetupProjectRenewalRoutes`'s style:

- `POST /api/v1/admin/boards/:boardId/jira-integration` — body
  `{ base_url, email, api_token, project_key }`. Before saving, performs
  a live test call to `GET {base_url}/rest/api/3/myself` using the
  supplied Basic Auth credentials; on failure returns 400 with the Jira
  error message and does not persist anything. On success, upserts the
  `jira_board_integrations` row for that board (one integration per
  board — a second `POST` updates the existing row rather than erroring).
- `GET /api/v1/admin/boards/:boardId/jira-integration` — returns
  `{ connected: bool, base_url?, email?, project_key?, last_sync_at?,
  last_sync_error? }`. `api_token` is never included in any response
  (`json:"-"` on the model field, same as `GmailAccount.AccessToken`).
- `DELETE /api/v1/admin/boards/:boardId/jira-integration` — deletes the
  `jira_board_integrations` row. Already-mirrored tasks/columns on the
  board are left as-is (they simply stop being updated — see
  "Non-goals").

### Sync job (`backend/internal/services/jira_sync.go`)

Same shape as `scheduler.go`: a ticker firing every 15 minutes, plus one
run immediately on startup.

`RunJiraSync()` loads all `jira_board_integrations` rows and processes
each independently (one board's failure doesn't stop the others):

1. Call the Jira Issue Search API (`GET {base_url}/rest/api/3/search/jql`,
   Basic Auth `email:api_token`) with JQL
   `project = {project_key} AND assignee = currentUser() AND statusCategory != Done`,
   paging through results.
2. For each returned issue:
   - Upsert the `KanbanColumn` for this board where
     `jira_status_name = issue.status.name`, creating it (appended after
     the board's existing columns, by current max position + 1) if this
     status name hasn't been seen yet on this board.
   - Upsert the `Task` keyed by `jira_issue_key` (scoped to this board's
     tasks): set `title = issue.summary`, `description =
     issue.description` (plain-text rendering of Jira's ADF format),
     `source = "jira"`, `jira_synced_at = now`.
   - Upsert the `TaskPlacement` so the task sits in the column matching
     its current Jira status (moving it if the status changed since the
     last sync).
3. For any previously-mirrored task on this board (`source = "jira"`)
   whose `jira_issue_key` was **not** present in this run's results
   (reassigned away from the connected account, or moved to a Done
   status) — delete the task (and its placement/time entries cascade as
   the existing task-delete path already handles). This is safe because
   these tasks are read-only mirrors; nothing local is ever attached to
   them that would need preserving beyond time entries, which are
   accepted as lost on unassignment/completion, consistent with this
   being a live filtered view rather than an archive.
4. On success: `last_sync_at = now`, `last_sync_error = nil`. On failure
   (network error, 401/403 from Jira, etc.): `last_sync_error =
   <message>`, `last_sync_at` unchanged, move on to the next board.

### `backend/internal/services/jira_client.go`

A small client interface for test-double injection, matching
`ClientStatusDrafter`'s shape:

```go
type JiraClient interface {
    TestConnection(ctx context.Context, baseURL, email, apiToken string) error
    SearchAssignedIssues(ctx context.Context, integration models.JiraBoardIntegration) ([]JiraIssue, error)
}
```

The concrete implementation does the real HTTP calls (Basic Auth header,
JSON decode, pagination via the search API's `nextPageToken`).

### Read-only enforcement

On the existing task/column handlers
(`UpdateTask`, `MoveTask`/`PlaceTask`, `DeleteTask` for tasks;
`UpdateColumn`, and the reorder/delete paths for columns), a guard added
at the top of each handler:

- Task handlers: if `task.Source == "jira"`, return 403 with
  `"Jira-eredetű feladat nem szerkeszthető ebben az appban"`.
- Column handlers: if `column.JiraStatusName != nil`, return 403 with the
  equivalent message for columns.

**Not guarded** (remain fully usable on Jira-sourced tasks): time entry
endpoints (`CreateTimeEntry`/`UpdateTimeEntry`/`DeleteTimeEntry`) and
comment endpoints (`CreateComment`/`UpdateComment`/`DeleteComment`) — both
stay local-only, never sent to Jira, per the approved design.

### New dependencies

None. Jira Cloud's REST API is plain JSON over HTTPS with Basic Auth;
implemented with the standard library `net/http`, exactly like
`jira_sync`'s sibling services already do for the AI microservice calls.

### New environment variables

None — each board's Jira credentials are supplied by the super admin
through the connection endpoint and stored in `jira_board_integrations`,
not read from process environment.

## Frontend

- **Board settings panel:** a "Jira integráció" section in the board's
  settings (super admin only, `isSuperAdmin(user)` gated, matching the
  existing dashboard widgets' gating style).
  - Not connected: a form (Base URL, Email, API token, Projekt kulcs) +
    "Kapcsolódás" button calling the `POST .../jira-integration`
    endpoint; shows the returned error inline on failure.
  - Connected: a status line (`Kapcsolva: {project_key} — utolsó
    szinkron: {last_sync_at}`, red error text if `last_sync_error` is
    set) + "Leválasztás" button.
- **Board view:**
  - Jira-sourced task cards show a small Jira icon/badge.
  - The task detail modal disables the title/description fields and the
    column-move control for `source === "jira"` tasks, with a caption
    ("Jira-ból szinkronizálva, csak Jirában szerkeszthető"); the time
    entry and comment sections remain fully interactive.
  - Drag-and-drop is disabled for Jira-sourced cards (`onDragStart`
    no-ops when `source === "jira"`), and Jira-derived columns
    (`jiraStatusName` set) can't be deleted, renamed, or reordered from
    the UI.

## Security

- The Jira API token is stored as a plaintext column in
  `jira_board_integrations`, matching this codebase's existing posture
  for `billingo_settings.api_key` and `gmail_accounts.access_token` — no
  secrets-manager or column encryption exists anywhere in this app, and
  introducing one solely for this feature would be new infrastructure
  disproportionate to the rest of the codebase's security posture. If
  this changes later, all three tables migrate together.
- All `/api/v1/admin/boards/:boardId/jira-integration*` routes require
  `super_admin` (`middleware.RequireRole("super_admin")`), consistent
  with the other admin-only dashboard features in this codebase.
- The Jira API token never reaches the browser; all Jira API calls
  happen server-side (connection test and sync job alike).

## Testing plan

- Backend: unit tests for the sync job's upsert/delete logic (column
  creation on first-seen status, task upsert by `jira_issue_key`,
  placement moves on status change, deletion when an issue drops out of
  the filtered result set) using a fake `JiraClient`, so the real Jira
  API is never hit in tests. `go build`/`go vet`/`gofmt` run in the
  documented backend container, as with prior work in this codebase.
- The read-only guards get handler tests confirming a 403 on
  `source == "jira"` tasks/`jira_status_name != nil` columns, and a
  passing 2xx on the same tasks for time-entry/comment endpoints.
- The real Jira connection test (`TestConnection` against an actual Jira
  Cloud site) cannot be driven non-interactively in this environment;
  the user performs a real connect using their own Jira Cloud
  credentials once implementation is ready, and confirms the
  connect/status/disconnect flow and that sync populates mirrored
  tasks — per this project's standing rule against touching external
  providers without explicit approval for each such action.
