# Unified Task Detail View (Description/Comments/History/Attachments) + Activity Log — Design

Date: 2026-09-04
Status: Approved by user, implementing.

## Goal

1. Replace the current split UX — a separate "edit" modal (`task-form.tsx`) and a separate
   "comments" modal (`comment-section.tsx`), opened by two different icons on the task card —
   with a single `TaskDetailModal` that has 4 tabs: **Description**, **Attachments**, **Comments**,
   **History**.
2. Add a real activity-log feature (`task_activity_log`), since none exists today: track who
   changed what on a task (field edits, column moves, assignee changes, comment/attachment
   add/delete) and surface it as a read-only timeline in the History tab.
3. Add an assignee field to tasks: the `assigneeId` column and backend support already exist
   (`UpdateTask` already accepts it), but there is currently no UI to set it anywhere. Add a
   dropdown, backed by the existing `GET /api/v1/projects/:id/assignments` endpoint (project
   members), so assignee changes are actually possible and thus loggable.
4. Convert the Description tab's save behavior from one whole-form "Save" button to per-field
   auto-save (save on blur for text fields, save on change for select/date/priority/tags), each
   field getting its own inline saving/saved/error indicator.

## Non-goals

- No live/real-time push updates while the modal is open (matches existing comments/attachments
  behavior — no websocket infra exists or is being introduced).
- No ability to revert/restore a previous value from History — it's a read-only audit trail, not
  version control.
- No new permission model — History visibility piggybacks on existing task-access checks.

## Data model

### `task_activity_log` table (new)

- `id`, `task_id` FK → tasks (cascade delete, mirrors `task_comments`/`task_attachments`)
- `user_id` FK → users (who made the change)
- `event_type`: `field_changed` | `moved` | `comment_added` | `comment_deleted` |
  `attachment_added` | `attachment_deleted`
- `field_name` (nullable, only for `field_changed`, e.g. `title`, `description`, `priority`,
  `due_date`, `estimated_hours`, `tags`, `assignee`)
- `old_value`, `new_value` (nullable text — human-readable display values, e.g. resolved user
  names for `assignee`, column names for `moved`; not typed/structured, since this is a display
  timeline, not a mechanism for reverting changes)
- `created_at`

No `.down.sql` — matches this repo's existing migration convention (000006–000010 have none
either).

### `Task` / `assigneeId`

No schema change — `assignee_id` already exists on `tasks` and `UpdateTaskRequest.AssigneeID` is
already a pointer field the backend already applies. Only new work is the frontend UI to set it.

## Backend

Mirrors the codebase's existing per-concern-handler convention; every write site below is an
addition to a handler that already exists and is already tested, not a new flow.

- `internal/models/activity_log.go` — GORM struct + `ActivityLogResponse` DTO (mirrors
  `attachment.go`'s shape).
- `internal/handlers/activity_log_handler.go` — `GetTaskHistory` (`GET /tasks/:id/history`,
  paginated via `limit`/`offset` like `GetComments`). Read-only; same task-access check as
  `GetTask`/`GetComments` (no new permission).
- `internal/routes/activity_log_routes.go`, registered in `routes.go`.
- A small internal helper `logActivity(taskID, userID, eventType, fieldName, oldVal, newVal)` in
  the handlers package, called from:
  - `task_handler.go`'s `UpdateTask` — after applying each non-nil pointer field from the
    request, diff old vs new and write one `field_changed` row per changed field (including
    `assignee`, resolving `AssigneeID` to the user's name before storing).
  - `task_handler.go`'s `MoveTask` — one `moved` row, `old_value`/`new_value` = column names.
  - `task_comment_handler.go`'s Create/Delete — one `comment_added`/`comment_deleted` row.
  - `attachment_handler.go`'s Upload/Delete — one `attachment_added`/`attachment_deleted` row per
    file (upload can be multi-file, so one log row each, matching the existing one-DB-row-per-file
    pattern already used for attachments themselves).
  - Migration `000014_create_task_activity_log_table.up.sql`.

## Frontend

### `TaskDetailModal` (new, replaces the two-modal split)

- `kanban-board.tsx`: the current two pieces of state driving `openEditModal`/`openCommentsModal`
  collapse into one `openTaskDetail(taskId, initialTab)`. The task card's edit icon calls it with
  `initialTab='description'`; the comment icon calls it with `initialTab='comments'`. The
  time-tracker icon/flow is untouched — out of scope, stays exactly as it is today.
- Tabs, each essentially today's component moved under one shell:
  - **Description** — today's `task-form.tsx` fields (title, description, priority, due date,
    estimated hours, tags) plus the new assignee dropdown, converted to per-field auto-save (see
    below).
  - **Attachments** — today's `attachment-list.tsx`, unchanged, just relocated out of the
    Description tab into its own tab.
  - **Comments** — today's `comment-section.tsx`, unchanged.
  - **History** — new: reverse-chronological list from `GET /tasks/:id/history`, paginated with a
    "load more" button (same pattern as comments' pagination), rendering
    `"{user} changed {field}: {old} → {new} — {timestamp}"` (or the equivalent phrasing per
    `event_type`).

### Per-field auto-save (Description tab)

- Each field gets its own save trigger: blur for text/number inputs, immediate `onChange` for
  select/date/tags: title, description, priority, due date, estimated hours, tags, assignee.
- Each field gets its own local `saving | saved | error` indicator (small inline icon/spinner next
  to the field) — replacing the current whole-form `isLoading`/error banner for this tab.
- Each save is a call to the existing `PUT /projects/:id/tasks/:taskId` (`UpdateTask`) endpoint
  with only that one field populated in the request body (the backend already supports this via
  pointer fields — no backend request-shape or route change needed).
- Tags keep their existing add/remove-triggers-immediate-local-state behavior, but now each
  add/remove also fires the same auto-save call with the full updated tag array (matching how
  `UpdateTaskRequest.Tags` already replaces the whole array, not per-tag).

### Assignee dropdown

- New control on the Description tab, options populated from the already-existing
  `GET /api/v1/projects/:id/assignments` (project members) — no new backend endpoint.
- Auto-saves like every other field on change.

## Testing

- Backend: `go build ./...`, `go vet ./...` in `devbridge_backend`; live API verification via curl
  against the running compose stack (create task, update single fields, move columns, add/delete
  comments and attachments, assign/reassign) confirming one `task_activity_log` row per action
  with correct `event_type`/`field_name`/`old_value`/`new_value`, plus `GET /tasks/:id/history`
  pagination and ordering.
- Frontend: `npx tsc --noEmit` in `devbridge_frontend`; no Playwright available in this
  environment (confirmed in the prior task-attachments work) — substitute curl-based API
  verification for backend-observable behavior, and rely on direct code review for pure
  client-side rendering/state assertions, as was done for the attachments feature.
