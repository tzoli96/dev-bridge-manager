# Multi-Board Architecture Design

Date: 2026-09-03

## Context

The kanban feature (built earlier this session) has no `Board` entity —
`kanban_columns` hangs directly off `project_id`, so each project has exactly
one implicit board. The user wants:

1. The top-level "Board" nav item relabeled to "Projects".
2. Multiple boards per project (e.g. "Sprint 12", "Bug Triage" within one project).
3. A task able to be placed on more than one board within the same project,
   sharing all its data (title, description, comments, attachments, estimate,
   time entries) across placements — only its column/position differs per board.

Two other items from the original request are already fully implemented and
out of scope here: rich-text comments (`rich-text-editor.tsx`, `HTMLContent`
on comments) and task estimation (`EstimatedHours` end-to-end). File
attachments on tasks are a separate, independent subsystem tracked as its own
design (not covered by this spec).

## Goals

- Introduce `Board` as a first-class entity, many per project.
- Decouple a task's shared data from its placement on a board: a task can
  have zero, one, or many placements (one per board, never two placements on
  the same board).
- Preserve all existing single-board behavior and data via migration — no
  manual fix-up required after deploy.
- Reuse existing conventions: same DTO-wrapper style (raw entity/array
  responses), same string-ID serialization, same permission model, same
  migration auto-run mechanism.

## Non-goals

- Per-placement divergence of task fields (e.g. different priority per
  board). All shared fields stay shared; only column/position vary.
- File attachments (separate design).
- New permissions — board CRUD reuses the existing `kanban.manage_columns`
  permission, since it's the same trust level as column management.

## Data model

### `boards` (new)

| column | type | notes |
|---|---|---|
| id | uint PK | |
| project_id | uint FK → projects, indexed | |
| name | string | |
| position | int | ordering among a project's boards |
| created_at / updated_at | timestamp | |

### `kanban_columns` (changed)

- Drop `project_id`.
- Add `board_id` (FK → boards, indexed, not null).

### `task_placements` (new)

| column | type | notes |
|---|---|---|
| id | uint PK | |
| task_id | uint FK → tasks, indexed | |
| board_id | uint FK → boards, indexed | denormalized for a simple uniqueness constraint and cheap board-scoped queries |
| column_id | uint FK → kanban_columns, indexed | |
| position | int | ordering within the column |
| created_at / updated_at | timestamp | |

Unique constraint on (`task_id`, `board_id`) — a task can appear at most once
per board.

### `tasks` (changed)

- Drop `column_id` and `position` — these are now placement-scoped, not
  task-scoped. Everything else (title, description, priority, tags,
  estimated hours, comments, time entries) is unchanged and remains shared
  across all of a task's placements.

## API changes

All routes are under the existing `api.Group("/projects")` with
`middleware.JWTMiddleware()`.

**Board CRUD** (new, permission: `kanban.manage_columns`):
- `GET /projects/:id/boards` — list boards for a project
- `POST /projects/:id/boards` — create a board; seeds the same 4 default
  columns (`To Do`/`In Progress`/`Review`/`Done`) used today for a project's
  first board
- `PUT /projects/:id/boards/:boardId` — rename / reposition
- `DELETE /projects/:id/boards/:boardId` — delete a board; cascades to its
  columns and their placements. A task whose deleted placement was its last
  one is fully deleted (comments/attachments/time entries cascade via
  existing FKs); otherwise it survives on its remaining boards.

**Board-scoped routes** (existing routes, gain `:boardId` in the path in
place of the implicit "the project's one board"):
- `GET/PUT /projects/:id/boards/:boardId/kanban` — board detail
- Column CRUD/reorder: `.../boards/:boardId/kanban/columns...`
- `PUT /projects/:id/boards/:boardId/tasks/:taskId/move` — move within/across
  columns of that board (operates on the placement, not the task)
- Comments and time-entries: unchanged in shape, just reached via
  `:boardId` scoped task routes

**Placement management** (new):
- `POST /projects/:id/boards/:boardId/tasks/:taskId/place` — add an existing
  task (from anywhere in the project) to this board/column as a new
  placement
- `DELETE /projects/:id/boards/:boardId/tasks/:taskId` — remove the task's
  placement from this board only. If this was the task's last placement, the
  task itself (and its comments/time entries) is deleted; otherwise only the
  placement row is removed and the task survives on its other boards.

**Project-scoped task routes** (new):
- `GET /projects/:id/tasks` — every task in the project, board-independent;
  backs the "add existing task to this board" picker
- `POST /projects/:id/tasks` — create a task; requires `boardId` + `columnId`
  in the body, creates the task row plus its first placement in one call
- `DELETE /projects/:id/tasks/:taskId` — delete the task everywhere,
  regardless of how many placements it has (explicit "delete permanently",
  distinct from the per-board removal above)

## Frontend changes

- `DashboardNav.tsx`: nav label "Board" → "Projects" (route path unchanged).
- `/dashboard/board/page.tsx`: unchanged behavior (still lists projects),
  heading text may need to follow the nav rename.
- `/dashboard/board/[projectId]/page.tsx`: **repurposed** — was the kanban
  view, becomes a boards-list page for that project (board cards + "New
  board" action).
- **New** `/dashboard/board/[projectId]/[boardId]/page.tsx`: the actual
  kanban view — the existing `KanbanBoard` component moves here, now reading
  `boardId` from the route in addition to `projectId`.
- `kanban-provider.tsx` / `kanban.service.ts`: all board-scoped calls gain
  `boardId`.
- Task card: "Remove from this board" (today's delete action, now scoped to
  the placement) plus a new "Add to another board" action opening a
  board+column picker that calls the `place` endpoint.
- Task edit modal: new explicit "Delete permanently" action, styled as a
  destructive/danger action distinct from the per-board removal.
- Column settings modal: the existing delete-column confirmation copy
  ("this will delete N task(s)") is updated to reflect placement semantics
  (a task is only actually destroyed if the column being deleted holds its
  last placement).

## Migration plan

`000012_create_boards_and_placements.up.sql`:
1. Create `boards`.
2. For each distinct `project_id` currently in `kanban_columns`, insert one
   `boards` row named `"Main Board"` (`position = 0`).
3. Add `kanban_columns.board_id`, backfill it from the new boards rows via
   the old `project_id`, then drop `kanban_columns.project_id`.
4. Create `task_placements`.
5. For every existing task, insert one `task_placements` row copying its
   current `column_id`/`position`, deriving `board_id` from that column.
6. Drop `tasks.column_id` and `tasks.position`.

Down migration reverses the shape (recreates the dropped columns) and
backfills each task from its first placement — an acceptable simplification
for local dev data, documented as a limitation rather than engineered for
lossless rollback of a task with multiple placements.

## Testing

- Backend: `go build ./...` plus targeted handler tests for placement
  creation, cross-board move, cascade delete (both "removed from last board"
  and "board deleted while task placed elsewhere" paths).
- End-to-end: Playwright pass creating a second board, adding an existing
  task to it, moving it independently on each board, removing it from one
  board (confirm it still exists on the other), and deleting it permanently
  (confirm it disappears from all boards) — same rigor as the column
  settings verification done earlier this session.
