# Subtasks — Design

Date: 2026-09-16
Status: Approved by user, implementing.

## Goal

Let a task be broken down into subtasks that are full, independent `Task` records — not a
lightweight checklist. A subtask is a real kanban card, placeable on any board within the same
project, with its own assignee/priority/due date/board-column position. The only thing that ties
it to its parent is a `parent_task_id` link, a "Subtasks" tab on the parent's `TaskDetailModal`,
and a visual badge on both cards.

## Non-goals

- No multi-level nesting — a subtask cannot itself have subtasks (enforced both in the UI and the
  backend). This is a hard, permanent constraint, not a v1 simplification.
- No converting an existing, already-created task into a subtask (or vice versa) — the
  parent/child link is set once, at subtask-creation time, via the "Add Subtask" flow, and is
  immutable afterward. No UI or API path re-parents a task.
- No cross-project subtasks — a subtask must live in the same project as its parent (board within
  that project is free to choose).
- No automatic parent state changes when all subtasks reach the Done column (e.g. no
  auto-complete/auto-move of the parent). The progress badge is informational only.

## Data model

### `tasks.parent_task_id` (new column)

- Nullable `uint`, FK → `tasks.id`, `ON DELETE CASCADE` — deleting a parent task deletes its
  subtasks. This mirrors the existing cascade convention used by `task_comments`/`task_attachments`
  pointing at `tasks`.
- Backend-enforced invariant: a row that is itself somebody's subtask (`parent_task_id IS NOT
  NULL`) can never be used as a `parent_task_id` value for another row — i.e. the chain is at
  most one level deep.

### `kanban_columns.is_done` (new column)

- `bool`, default `false`. Marks a column as the board's "Done" column, used purely to compute
  subtask-completion progress (see below). At most one column per board may have `is_done = true`;
  setting it on one column clears it on any other column of the same board.

Migration: `backend/migrations/000015_add_subtasks_and_done_column.up.sql` /
`.down.sql` (drops both columns), following the `.down.sql`-included convention already
established for the kanban/attachments migration family (000011–000014).

## Backend

Extends existing handlers and DTOs — no new handler file, no new architectural layer.

- `internal/models/kanban.go`:
  - `Task.ParentTaskID *uint`.
  - `KanbanColumn.IsDone bool`.
  - `CreateTaskRequest` gets `ParentTaskID *uint`.
  - `TaskDTO` gets two new response-only fields:
    - `subtaskProgress *SubtaskProgressDTO` (`{ total int; done int }`), populated only when the
      task has subtasks.
    - `parentTask *TaskParentRefDTO` (`{ id string; title string }`), populated only when the task
      is itself a subtask.
  - `KanbanColumnDTO` gets `isDone bool`.
- `internal/handlers/task_handler.go`:
  - `CreateTask` — if `ParentTaskID` is set: load the referenced task, reject with 400 if it
    doesn't exist, isn't in the same project, or already has a non-null `parent_task_id` of its
    own (enforces the one-level rule). On success, call `logActivity` on the **parent** task with
    a new `subtask_added` event type (child id/title as the value), reusing the helper wired in by
    the task-detail-history feature.
  - `UpdateTask` — `parent_task_id` is never read from `UpdateTaskRequest`; there is no field for
    it, so it cannot be changed post-creation.
  - `DeleteTask` — unchanged; the DB-level `ON DELETE CASCADE` removes subtasks automatically. No
    extra activity-log rows are written for the cascaded deletions (the parent row, and its
    history, are gone too).
  - New handler function `GetSubtasks` — `GET /api/v1/projects/:id/tasks/:taskId/subtasks`,
    returns `[]TaskDTO` for all tasks with `parent_task_id = :taskId`, same task-access check as
    `GetTask`. Registered in `internal/routes/kanban_routes.go` alongside the existing task routes
    (per this repo's "one routes file for all kanban-adjacent routes" convention).
- `internal/handlers/kanban_dto_builder.go`:
  - When building `TaskDTO`s for a board response, batch-compute `subtaskProgress` for every task
    that has subtasks with a single grouped query (subtask count + count where the subtask's
    current column has `is_done = true`, grouped by `parent_task_id`) — not one query per task.
  - When a task has a `parent_task_id`, resolve `parentTask.title` via a join on the parent row
    (title only, no placement/column data needed since the parent may be on a different board).
- `internal/handlers/kanban_handler.go`:
  - `UpdateColumnRequest` gets `IsDone *bool`. When a column is updated with `IsDone = true`, the
    handler clears `is_done` on every other column of the same board in the same transaction.

## Frontend

- `frontend/src/components/kanban/task-card.tsx`:
  - If `subtaskProgress` is present: small "done/total" badge with a checklist icon.
  - If `parentTask` is present: small "↳ {parentTask.title}" badge; clicking it opens that task's
    `TaskDetailModal` by id. This works across boards without navigation, because task fetching
    (`GET /projects/:id/tasks/:taskId`) is already board-independent.
- `frontend/src/components/kanban/task-detail/TaskDetailModal.tsx`:
  - New "Subtasks" tab, added to the existing tab bar alongside Description/Attachments/Comments/
    History. Hidden entirely when the open task itself has a `parentTask` (a subtask can't have
    subtasks — enforced in the UI too, not just the backend).
- `frontend/src/components/kanban/task-detail/SubtasksTab.tsx` (new):
  - Lists subtasks fetched from `GET .../subtasks`: title, assignee avatar, a Done/not-done badge
    derived from the subtask's current column `isDone` flag. Clicking a row opens that subtask's
    own `TaskDetailModal` (same board-independent open-by-id mechanism as the parent badge).
  - "Add Subtask" control at the top: a small form (title, board/column picker scoped to the
    current project, optional assignee/priority/due date), reusing `task-form.tsx`'s field
    components as a thin wrapper, matching how the other tabs reuse existing components rather
    than re-implementing fields.
- `frontend/src/components/kanban/column-settings-modal.tsx`:
  - New "Done column" toggle. If enabling it while another column on the board is already marked
    Done, show an inline note that the previous one will be unmarked (matches the backend's
    single-done-column enforcement).
- `frontend/src/components/kanban/kanban-board.tsx`:
  - Passes the new `subtaskProgress`/`parentTask` `TaskDTO` fields through to `task-card`; opening
    a task-detail modal for a different task id (from a badge click) reuses the existing
    `activeModal` state mechanism, just targeting a different task id.
- `frontend/src/types/kanban/*`, `frontend/src/services/kanban/*`, `frontend/src/hooks/kanban/*`:
  add/extend the barrel-exported types, a `getSubtasks`/`createSubtask` service function, and a
  `useSubtasks(projectId, taskId)` hook — mirroring the existing per-concern service/hook pattern
  (`activity-log.service.ts` / `use-activity-log.ts` from the previous feature).

## Testing

- Backend: run the existing Go test suite inside `devbridge_backend` (`docker exec devbridge_backend go test ./...`); add/extend handler tests for `CreateTask` with `parentTaskId` (success, cross-project rejection, two-level rejection), `GetSubtasks`, and the single-Done-column enforcement on `UpdateColumn`.
- Frontend: `docker exec devbridge_frontend npx tsc --noEmit` for type-checking; no Playwright available in this environment (matches the prior task-detail-history and task-attachments work), so UI behavior is verified via direct code review plus curl-based API checks against the local compose stack for the new/changed endpoints.
- Any QA project/task/column created for manual verification against the local compose stack must be deleted afterward — no production/shared data involved at any point.
