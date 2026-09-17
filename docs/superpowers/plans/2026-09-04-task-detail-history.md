# Implementation Plan: Unified Task Detail View + Activity Log

## Goal

Implement the approved design spec: replace the split TaskForm/CommentSection modals with
one `TaskDetailModal` (Description / Attachments / Comments / History tabs), add a
`task_activity_log` table + read-only History feed, add an assignee dropdown, and convert
the Description tab to per-field auto-save.

## Architecture

Backend: Go/Fiber/GORM, one handler file per concern (existing convention: `task_handler.go`,
`task_comment_handler.go`, `attachment_handler.go` all live under `internal/handlers/`, all
registered together in `internal/routes/kanban_routes.go`). A new `logActivity` helper is
called from the four existing write-handlers — no new architectural layer.

Frontend: Next.js/TypeScript. `kanban-board.tsx` owns all modal instantiation via a local
`activeModal` state keyed by `ModalType`. `TaskForm`/`CommentSection` have no modal chrome of
their own (chrome is the shared `<Modal>` wrapper) — the new `TaskDetailModal` is a new modal
instance with its own tab bar; `TaskForm`/`CommentSection`'s internals are reused mostly
as-is inside new thin tab wrappers, not rewritten.

## Tech Stack

Go 1.x / Fiber v2 / GORM / PostgreSQL (backend, container `devbridge_backend`), Next.js /
React / TypeScript / Zustand (frontend, container `devbridge_frontend`), both live via
`docker compose` (`devbridge_backend`, `devbridge_frontend`, `devbridge_postgres`).

## Spec

`docs/superpowers/specs/2026-09-04-task-detail-history-design.md`

## Corrections to the spec (discovered during file-boundary research, applied below)

1. **Migration down-file convention**: the spec says "No `.down.sql` — matches 000006–000010".
   That's the wrong precedent — migrations 000006-000010 are an older, unrelated feature set.
   The kanban/attachments family this feature actually extends (000011, 000013) **does**
   include a `.down.sql`. Migration 000014 below includes one.
2. **History pagination "like GetComments"**: `GetComments` (`task_comment_handler.go:22`)
   has no pagination at all — it loads every comment unconditionally. There is no existing
   pagination precedent to mirror, so `GetTaskHistory` below implements straightforward
   `limit`/`offset` query params directly (default `limit=20`), and the frontend infers
   "more available" from `len(page) === limit`.
3. **Routes file**: the spec says a new `internal/routes/activity_log_routes.go`. The actual
   convention is that every kanban-adjacent route (tasks, comments, attachments) is
   registered in the single existing `internal/routes/kanban_routes.go` under
   `SetupKanbanRoutes`. The new route is added there instead — no new routes file, per the
   "extend established flows first" constraint.

## Global Constraints (binding for every task below)

- Run all Go/Node tooling only via `docker exec devbridge_backend ...` / `docker exec devbridge_frontend ...` — never on host.
- No new dependency, package, or infra component — everything below reuses `gofiber`, `gorm`, existing React/TS patterns already in the repo.
- No commits/pushes at any point unless explicitly requested — leave changes in the working tree.
- Live verification (curl / manual browser check) only against the local compose stack; no production/shared data.
- Any QA task/comment/attachment/user created for verification must be deleted afterward.

---

## File Structure

**Backend — create:**
- `backend/migrations/000014_create_task_activity_log_table.up.sql` — new table
- `backend/migrations/000014_create_task_activity_log_table.down.sql` — drop table
- `backend/internal/models/activity_log.go` — `TaskActivityLog` GORM model + `ActivityLogDTO`
- `backend/internal/handlers/activity_log_handler.go` — `logActivity` helper, `logTaskFieldChanges` helper, `GetTaskHistory` handler

**Backend — modify:**
- `backend/internal/handlers/task_handler.go` — `UpdateTask` (diff+log field changes), `MoveTask` (log column move)
- `backend/internal/handlers/task_comment_handler.go` — `CreateComment`, `DeleteComment` (log add/delete)
- `backend/internal/handlers/attachment_handler.go` — `UploadAttachments`, `DeleteAttachment` (log add/delete)
- `backend/internal/routes/kanban_routes.go` — register `GET /:id/tasks/:taskId/history`

**Frontend — create:**
- `frontend/src/types/kanban/activity-log.types.ts` — `ActivityLogEntry` type
- `frontend/src/services/kanban/activity-log.service.ts` — `getHistory(projectId, taskId, {limit, offset})`
- `frontend/src/hooks/kanban/use-activity-log.ts` — `useActivityLog(projectId)`
- `frontend/src/hooks/kanban/use-task-assignees.ts` — thin wrapper over existing `ProjectAssignmentService.getProjectAssignments`
- `frontend/src/components/ui/tabs.tsx` — minimal tab-bar primitive (none exists today)
- `frontend/src/components/kanban/task-detail/TaskDetailModal.tsx` — modal shell + tab switching
- `frontend/src/components/kanban/task-detail/DescriptionTab.tsx` — per-field auto-save form + assignee dropdown
- `frontend/src/components/kanban/task-detail/AttachmentsTab.tsx` — thin wrapper around existing `AttachmentList`
- `frontend/src/components/kanban/task-detail/CommentsTab.tsx` — thin wrapper around existing `CommentSection`
- `frontend/src/components/kanban/task-detail/HistoryTab.tsx` — paginated activity feed

**Frontend — modify:**
- `frontend/src/types/kanban/kanban.types.ts` — `ModalType` enum: remove `COMMENTS`, add `TASK_DETAIL`
- `frontend/src/components/kanban/kanban-board.tsx` — `activeModal` shape, `handleEditTask`/`handleOpenComments`, modal render block
- `frontend/src/components/ui/rich-text-editor.tsx` — add optional `onBlur` prop (needed for per-field auto-save on the description field)
- `frontend/src/hooks/kanban/index.ts`, `frontend/src/services/kanban/index.ts`, `frontend/src/types/kanban/index.ts` (if barrel files exist — verify at Task 6/9 and export the new modules)

**Frontend — unmodified but reused as-is:** `task-form.tsx` (stays for create-mode only), `comment-section.tsx`, `attachment-list.tsx`, `use-comments.ts`, `use-attachments.ts`, `use-tasks.ts`.

---

## Task 1: Migration + `TaskActivityLog` model

### Files
- Create: `backend/migrations/000014_create_task_activity_log_table.up.sql`
- Create: `backend/migrations/000014_create_task_activity_log_table.down.sql`
- Create: `backend/internal/models/activity_log.go`
- Test: none (schema-only; verified in Task 3 via live insert)

### Interfaces
- Produces: `task_activity_log` table; `models.TaskActivityLog` (GORM struct), `models.ActivityLogDTO` (JSON response shape, camelCase — matches the kanban module's convention, e.g. `TaskDTO`/`TaskCommentDTO`, not the snake_case used by the unrelated clients/project module).

### Steps

1. Create the up migration, modeled on `backend/migrations/000013_create_attachments_table.up.sql`'s FK/cascade/index pattern:

```sql
-- backend/migrations/000014_create_task_activity_log_table.up.sql
CREATE TABLE task_activity_log (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    event_type VARCHAR(30) NOT NULL,
    field_name VARCHAR(50),
    old_value TEXT,
    new_value TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_activity_log_task_id ON task_activity_log(task_id);
```

2. Create the down migration:

```sql
-- backend/migrations/000014_create_task_activity_log_table.down.sql
DROP TABLE IF EXISTS task_activity_log;
```

3. Create `backend/internal/models/activity_log.go`:

```go
package models

import "time"

type TaskActivityLog struct {
	ID        uint `gorm:"primaryKey"`
	TaskID    uint `gorm:"not null;index"`
	UserID    uint `gorm:"not null"`
	EventType string
	FieldName *string
	OldValue  *string `gorm:"type:text"`
	NewValue  *string `gorm:"type:text"`
	CreatedAt time.Time
}

func (TaskActivityLog) TableName() string { return "task_activity_log" }

type ActivityLogDTO struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"taskId"`
	UserID    string          `json:"userId"`
	User      *TaskUserRefDTO `json:"user,omitempty"`
	EventType string          `json:"eventType"`
	FieldName string          `json:"fieldName,omitempty"`
	OldValue  string          `json:"oldValue,omitempty"`
	NewValue  string          `json:"newValue,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}
```

4. Apply the migration inside the backend container using this repo's existing migration
   mechanism (check `backend/cmd/` or `Makefile`/`docker-compose` entrypoint for how
   000001-000013 were applied — likely an automatic migrate-on-boot in `main.go`, given no
   manual migrate command has appeared anywhere else this session; confirm by reading
   `backend/cmd/main.go` or `backend/internal/database/` before assuming, then restart the
   backend container so it runs) — do not hand-run SQL against the container's postgres
   unless the automatic path doesn't exist.

### Verification
`docker exec devbridge_backend go build ./...` succeeds. `docker exec devbridge_postgres psql -U <user> -d <db> -c "\d task_activity_log"` shows the new table with the FK/index.

---

## Task 2: `logActivity` helper + `GetTaskHistory` handler + route

### Files
- Create: `backend/internal/handlers/activity_log_handler.go`
- Modify: `backend/internal/routes/kanban_routes.go` (add one line inside `SetupKanbanRoutes`, next to the existing `projects.Get("/:id/tasks/:taskId/comments", ...)` at line 48)

### Interfaces
- Produces: `GET /api/v1/projects/:id/tasks/:taskId/history?limit=20&offset=0` → `[]models.ActivityLogDTO`, newest first.
- Produces (internal, consumed by Tasks 3-4): `logActivity(taskID, userID uint, eventType, fieldName, oldValue, newValue string)`.
- Consumes: `database.GetDB()`, `loadUsersByIDs` (already in `kanban_dto_builder.go:10`), `models.IDToStr`, `models.TaskUserRefDTO`.

### Steps

1. Create `backend/internal/handlers/activity_log_handler.go`:

```go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type ActivityLogHandler struct{}

func NewActivityLogHandler() *ActivityLogHandler {
	return &ActivityLogHandler{}
}

// logActivity is a fire-and-forget write shared by task/comment/attachment handlers.
// Empty strings for fieldName/oldValue/newValue are stored as NULL.
func logActivity(taskID, userID uint, eventType, fieldName, oldValue, newValue string) {
	entry := models.TaskActivityLog{TaskID: taskID, UserID: userID, EventType: eventType}
	if fieldName != "" {
		entry.FieldName = &fieldName
	}
	if oldValue != "" {
		entry.OldValue = &oldValue
	}
	if newValue != "" {
		entry.NewValue = &newValue
	}
	if err := database.GetDB().Create(&entry).Error; err != nil {
		log.Printf("⚠️ Failed to log activity for task %d: %v", taskID, err)
	}
}

// GetTaskHistory - GET /api/v1/projects/:id/tasks/:taskId/history
func (h *ActivityLogHandler) GetTaskHistory(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	var entries []models.TaskActivityLog
	if err := database.GetDB().
		Where("task_id = ?", taskID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading history"})
	}

	userIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	users := loadUsersByIDs(userIDs)

	dtos := make([]models.ActivityLogDTO, 0, len(entries))
	for _, e := range entries {
		dto := models.ActivityLogDTO{
			ID:        models.IDToStr(e.ID),
			TaskID:    models.IDToStr(e.TaskID),
			UserID:    models.IDToStr(e.UserID),
			User:      userRefDTO(e.UserID, users),
			EventType: e.EventType,
			CreatedAt: e.CreatedAt,
		}
		if e.FieldName != nil {
			dto.FieldName = *e.FieldName
		}
		if e.OldValue != nil {
			dto.OldValue = *e.OldValue
		}
		if e.NewValue != nil {
			dto.NewValue = *e.NewValue
		}
		dtos = append(dtos, dto)
	}

	return c.JSON(dtos)
}
```

2. In `backend/internal/routes/kanban_routes.go`, add the handler instance and route next to
   the existing comment routes (immediately after line 48's
   `projects.Get("/:id/tasks/:taskId/comments", commentHandler.GetComments)`):

```go
activityLogHandler := handlers.NewActivityLogHandler()
projects.Get("/:id/tasks/:taskId/history", activityLogHandler.GetTaskHistory)
```

   (Read the file first to place `activityLogHandler := ...` next to the other
   `xHandler := handlers.NewXHandler()` declarations near the top of `SetupKanbanRoutes`,
   matching the existing `commentHandler`/`attachmentHandler` declaration style — do not
   redeclare inline.)

### Verification
`docker exec devbridge_backend go build ./... && go vet ./...`. Live: `curl -H "Authorization: Bearer $TOKEN" http://localhost:<port>/api/v1/projects/<id>/tasks/<taskId>/history` on an existing task returns `[]` (200, empty array, no rows yet — confirms route + auth wiring before Task 3 adds writers).

---

## Task 3: Wire `logActivity` into `UpdateTask` and `MoveTask`

### Files
- Modify: `backend/internal/handlers/task_handler.go` (`UpdateTask` at line 143, `MoveTask` at line 215)
- Modify: `backend/internal/handlers/activity_log_handler.go` (add `logTaskFieldChanges` + small diff helpers)

### Interfaces
- Consumes: `models.Task`, `models.KanbanColumn`, `models.FormatDate`, `loadUsersByIDs`.
- Produces: one `field_changed` row per changed field on `UpdateTask`; one `moved` row per `MoveTask` call.

### Steps

1. Add to `activity_log_handler.go` — the tag-name and field-diff helpers:

```go
import "encoding/json"
// (add alongside existing imports)

func tagNamesFromJSON(raw string) string {
	if raw == "" {
		return ""
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return ""
	}
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

func assigneeName(id *uint, users map[uint]models.User) string {
	if id == nil {
		return ""
	}
	if u, ok := users[*id]; ok {
		return u.Name
	}
	return ""
}

// logTaskFieldChanges diffs the task before/after UpdateTask's mutation and logs one
// field_changed row per field that actually changed.
func logTaskFieldChanges(original, updated models.Task, userID uint) {
	logField := func(field, oldVal, newVal string) {
		if oldVal == newVal {
			return
		}
		logActivity(updated.ID, userID, "field_changed", field, oldVal, newVal)
	}

	logField("title", original.Title, updated.Title)
	logField("description", original.Description, updated.Description)
	logField("priority", original.Priority, updated.Priority)
	logField("dueDate", models.FormatDate(original.DueDate), models.FormatDate(updated.DueDate))
	logField("estimatedHours",
		strconv.FormatFloat(original.EstimatedHours, 'f', -1, 64),
		strconv.FormatFloat(updated.EstimatedHours, 'f', -1, 64))
	logField("tags", tagNamesFromJSON(original.Tags), tagNamesFromJSON(updated.Tags))

	oldAssignee, newAssignee := original.AssigneeID, updated.AssigneeID
	changed := (oldAssignee == nil) != (newAssignee == nil)
	if !changed && oldAssignee != nil && newAssignee != nil {
		changed = *oldAssignee != *newAssignee
	}
	if changed {
		ids := make([]uint, 0, 2)
		if oldAssignee != nil {
			ids = append(ids, *oldAssignee)
		}
		if newAssignee != nil {
			ids = append(ids, *newAssignee)
		}
		users := loadUsersByIDs(ids)
		logField("assignee", assigneeName(oldAssignee, users), assigneeName(newAssignee, users))
	}
}
```

   Add `"strings"` to `activity_log_handler.go`'s import block alongside the `"encoding/json"`
   addition (`"strconv"` is already imported there from Task 2's `GetTaskHistory`).

2. In `task_handler.go`'s `UpdateTask` (line 143), capture the pre-mutation snapshot right
   after the task loads successfully (after line 152's closing `}`), and call the new helper
   after the successful `Save` (after line 195's closing `}`, before line 197's `return`):

```go
	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	original := task
	// ... (existing req parsing + field mutations, unchanged) ...
	if err := database.GetDB().Save(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating task"})
	}
	logTaskFieldChanges(original, task, currentUserID(c))

	return c.JSON(loadSingleTaskDTO(task, nil))
```

3. In `MoveTask` (line 215), after `db.Save(&task)` at line 276 and before the `return` at
   line 278, resolve both column titles and log one `moved` row:

```go
	task.UpdatedBy = currentUserID(c)
	db.Save(&task)

	var cols []models.KanbanColumn
	db.Where("id IN ?", []uint{oldColumnID, columnID}).Find(&cols)
	titleByID := make(map[uint]string, len(cols))
	for _, col := range cols {
		titleByID[col.ID] = col.Title
	}
	logActivity(task.ID, currentUserID(c), "moved", "", titleByID[oldColumnID], titleByID[columnID])

	return c.JSON(loadSingleTaskDTO(task, &placement))
```

   (`oldColumnID` and `columnID` are already in scope at lines 245/239.)

### Verification
`docker exec devbridge_backend go build ./... && go vet ./...`. Live: `PUT` a single-field
update (e.g. `{"priority":"high"}`) to an existing test task, then `GET .../history` — expect
exactly one `field_changed` row, `fieldName:"priority"`. Move the task to a different column
via the move endpoint, `GET .../history` again — expect one additional `moved` row with
correct old/new column names. Update an unrelated field with the same value twice — expect no
new row (no-op diff correctly skipped).

---

## Task 4: Wire `logActivity` into comment and attachment handlers

### Files
- Modify: `backend/internal/handlers/task_comment_handler.go` (`CreateComment` line 61, `DeleteComment` line 131)
- Modify: `backend/internal/handlers/attachment_handler.go` (`UploadAttachments` line 77, `DeleteAttachment` line 207)

### Steps

1. `CreateComment` — after `database.GetDB().Create(&comment)` succeeds (after line 83's
   closing `}`, before line 85's `users := ...`):

```go
	preview := comment.Content
	if len(preview) > 80 {
		preview = preview[:80] + "…"
	}
	logActivity(comment.TaskID, comment.UserID, "comment_added", "", "", preview)
```

2. `DeleteComment` (lines 131-152) currently deletes by ID alone, with no prior load, so
   `TaskID`/`Content` aren't in scope for the log call. Read the function first, then apply
   only this surgical change — do not alter any other existing behavior in the function (e.g.
   any existing attachment/file cleanup it already does, if any):
   - Immediately after parsing `commentID` and before the existing delete call, add:
     ```go
     var comment models.TaskComment
     database.GetDB().First(&comment, commentID)
     ```
   - Immediately after the existing delete call succeeds (before the function's existing
     `return`), add:
     ```go
     preview := comment.Content
     if len(preview) > 80 {
         preview = preview[:80] + "…"
     }
     logActivity(comment.TaskID, currentUserID(c), "comment_deleted", "", "", preview)
     ```
   - `currentUserID` is already defined package-wide (`task_handler.go:19`), reused as-is.

3. `UploadAttachments` (`attachment_handler.go:77`) — inside the per-file loop, right after
   `saved = append(saved, attachment)` (line 151):

```go
		saved = append(saved, attachment)
		logActivity(attachment.TaskID, userID, "attachment_added", "", "", attachment.OriginalName)
```

4. `DeleteAttachment` (line 207) — right after the successful `Delete` call (after line 229's
   closing `}`), before the `path := filepath.Join(...)` cleanup at line 231:

```go
	if err := database.GetDB().Delete(&attachment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting attachment"})
	}
	logActivity(attachment.TaskID, userID, "attachment_deleted", "", "", attachment.OriginalName)

	path := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(int(attachment.TaskID)), attachment.Filename)
```

### Verification
`docker exec devbridge_backend go build ./... && go vet ./...`. Live, on a disposable test
task: add a comment → history shows `comment_added` with a content preview; delete it →
`comment_deleted`; upload a file → `attachment_added` with the original filename; delete it →
`attachment_deleted`. Clean up the disposable task/comment/attachment afterward.

---

## Task 5: Backend full-suite check for this feature

### Steps
1. `docker exec devbridge_backend go build ./...`
2. `docker exec devbridge_backend go vet ./...`
3. Re-run every live-curl check from Tasks 2-4 back-to-back against one disposable test task
   (create → update several fields individually → move → comment → attachment → delete all),
   confirming `GET .../history?limit=20&offset=0` returns all rows newest-first, and
   `?limit=2` returns exactly 2 with the rest reachable via `offset=2`.
4. Delete the disposable task/comments/attachments/user created for this check.

Report exact commands run and their pass/fail output — this is the backend half's completion
gate before frontend work begins.

---

## Task 6: Frontend activity-log types, service, hook

### Files
- Create: `frontend/src/types/kanban/activity-log.types.ts`
- Create: `frontend/src/services/kanban/activity-log.service.ts`
- Create: `frontend/src/hooks/kanban/use-activity-log.ts`
- Modify: `frontend/src/types/kanban/index.ts`, `frontend/src/services/kanban/index.ts`, `frontend/src/hooks/kanban/index.ts` (barrel exports — read each first; `services/kanban/index.ts` already re-exports `comment.service.ts`, add the new one the same way)

### Interfaces
- Produces: `useActivityLog(projectId).loadHistory(taskId, {limit, offset}) => Promise<ActivityLogEntry[]>`.
- Consumes: `apiClient.get<T>` (`frontend/src/lib/api.ts`), same pattern as `comment.service.ts`.

### Steps

1. `activity-log.types.ts`, mirroring `TaskComment`'s shape in `task.types.ts:77-92`:

```ts
export interface ActivityLogEntry {
    id: string;
    taskId: string;
    userId: string;
    user?: {
        id: string;
        name: string;
        avatar?: string;
    };
    eventType: 'field_changed' | 'moved' | 'comment_added' | 'comment_deleted' | 'attachment_added' | 'attachment_deleted';
    fieldName?: string;
    oldValue?: string;
    newValue?: string;
    createdAt: string;
}
```

2. `activity-log.service.ts`, directly modeled on `comment.service.ts`'s
   `getComments(projectId, taskId)` (`apiClient.get(\`/projects/${projectId}/tasks/${taskId}/comments\`)`):

```ts
import { apiClient } from '@/lib/api';
import type { ActivityLogEntry } from '@/types/kanban';

export const activityLogService = {
    getHistory: (projectId: string, taskId: string, params?: { limit?: number; offset?: number }) => {
        const query = new URLSearchParams();
        if (params?.limit) query.set('limit', String(params.limit));
        if (params?.offset) query.set('offset', String(params.offset));
        const qs = query.toString();
        return apiClient.get<ActivityLogEntry[]>(
            `/projects/${projectId}/tasks/${taskId}/history${qs ? `?${qs}` : ''}`
        );
    },
};
```

   (Confirm `apiClient.get`'s exact generic signature in `frontend/src/lib/api.ts` before
   writing this — the fork's research pass confirmed `get<T>`, `post<T>`, `put<T>`,
   `delete<T>`, `uploadFiles<T>`, `getBlob` exist; match whichever of those returns the parsed
   body directly, as `comment.service.ts` does.)

3. `use-activity-log.ts`, modeled on `use-comments.ts`'s `loadComments` (full file quoted
   above in this plan's research — same `isLoading`/`error` local-state shape, but History is
   read-only so no store mutation helpers are needed, just local component state returned to
   the caller):

```ts
'use client';

import { useState, useCallback } from 'react';
import { activityLogService } from '@/services/kanban';
import type { ActivityLogEntry } from '@/types/kanban';

interface UseActivityLogReturn {
    isLoading: boolean;
    error: string | null;
    loadHistory: (taskId: string, params?: { limit?: number; offset?: number }) => Promise<ActivityLogEntry[]>;
}

export const useActivityLog = (projectId: string): UseActivityLogReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const loadHistory = useCallback(async (taskId: string, params?: { limit?: number; offset?: number }) => {
        setIsLoading(true);
        setError(null);
        try {
            return await activityLogService.getHistory(projectId, taskId, params);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load history');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId]);

    return { isLoading, error, loadHistory };
};
```

4. Add the new hook to `use-task-assignees.ts` in the same task's scope — new file:

```ts
'use client';

import { useState, useCallback } from 'react';
import { ProjectAssignmentService } from '@/services/projectAssignmentService';
import type { ProjectAssignment } from '@/services/projectAssignmentService';

export const useTaskAssignees = (projectId: string) => {
    const [assignees, setAssignees] = useState<ProjectAssignment[]>([]);
    const [isLoading, setIsLoading] = useState(false);

    const loadAssignees = useCallback(async () => {
        setIsLoading(true);
        try {
            const list = await ProjectAssignmentService.getProjectAssignments(Number(projectId));
            setAssignees(list.filter(a => a.is_active));
        } finally {
            setIsLoading(false);
        }
    }, [projectId]);

    return { assignees, isLoading, loadAssignees };
};
```

   (Verify `ProjectAssignmentService`'s exact export style — static class vs. instance — and
   `getProjectAssignments`'s exact parameter type, `number` vs `string`, against
   `frontend/src/services/projectAssignmentService.ts` before writing this file; the research
   pass confirmed the method name and the `ProjectAssignment` field names — `user_id`,
   `user_name` — but not the export shape.)

5. Update the three barrel `index.ts` files to export the new modules, matching each file's
   existing export style (read each before editing — do not assume a shape).

### Verification
`docker exec devbridge_frontend npx tsc --noEmit` — zero new errors.

---

## Task 7: Tabs primitive + `RichTextEditor` onBlur support

### Files
- Create: `frontend/src/components/ui/tabs.tsx`
- Modify: `frontend/src/components/ui/rich-text-editor.tsx`

### Steps

1. No Tabs component exists anywhere under `frontend/src/components/ui/` (confirmed by
   repo-wide search). Build a minimal, unstyled-beyond-Tailwind tab bar matching this
   codebase's existing small-component style (see `Modal`/`Select` above — plain function
   components, `cn()` from `@/lib/utils` for conditional classes, no external UI library):

```tsx
'use client';

import React from 'react';
import { cn } from '@/lib/utils';

export interface TabItem {
    id: string;
    label: string;
}

interface TabsProps {
    tabs: TabItem[];
    activeTab: string;
    onChange: (tabId: string) => void;
}

export const Tabs: React.FC<TabsProps> = ({ tabs, activeTab, onChange }) => {
    return (
        <div className="flex border-b border-gray-200 px-6">
            {tabs.map((tab) => (
                <button
                    key={tab.id}
                    type="button"
                    onClick={() => onChange(tab.id)}
                    className={cn(
                        'px-4 py-3 text-sm font-medium border-b-2 -mb-px transition-colors',
                        activeTab === tab.id
                            ? 'border-blue-600 text-blue-600'
                            : 'border-transparent text-gray-500 hover:text-gray-700'
                    )}
                >
                    {tab.label}
                </button>
            ))}
        </div>
    );
};
```

2. `rich-text-editor.tsx` currently has no `onBlur` prop (interface at line 7-12: `content`,
   `onChange`, `placeholder`, `minHeight` only), so blur-triggered auto-save can't hook into
   it as-is. Add an optional `onBlur`, wired to the underlying `contentEditable` element:

```tsx
interface RichTextEditorProps {
    content: string;
    onChange: (html: string, text: string) => void;
    onBlur?: () => void;
    placeholder?: string;
    minHeight?: string;
}

export const RichTextEditor: React.FC<RichTextEditorProps> = ({ content, onChange, onBlur, placeholder, minHeight = '100px' }) => {
```

   Then add `onBlur={onBlur}` to the `contentEditable` div's prop list (the element that
   currently has `onInput`/similar wired to `emitChange` — read the file's render section
   first to find the exact prop list on that element before inserting, since the exact
   attribute order wasn't re-verified byte-for-byte in this plan's research pass).

### Verification
`docker exec devbridge_frontend npx tsc --noEmit` — zero new errors. Manually confirm
`RichTextEditor` still renders/edits normally in the existing create-task flow (unchanged
behavior when `onBlur` is omitted).

---

## Task 8: `TaskDetailModal` shell

### Files
- Create: `frontend/src/components/kanban/task-detail/TaskDetailModal.tsx`

### Interfaces
- Consumes: `Modal` (`@/components/ui/modal`), `Tabs` (Task 7), `useTasks(projectId).getTask(taskId)` (`task.types.ts` `Task`).
- Produces: `<TaskDetailModal isOpen taskId projectId initialTab onClose />`, rendering one of the four tab components (Tasks 9-11) based on active tab state.

### Steps

1. Create the shell. It owns the active-tab state (seeded from `initialTab`, reset whenever
   `taskId` changes so re-opening on a different task doesn't keep a stale tab), and renders
   nothing but a `<Modal>` + `<Tabs>` + the active tab's component:

```tsx
'use client';

import React from 'react';
import { Modal } from '@/components/ui/modal';
import { Tabs, TabItem } from '@/components/ui/tabs';
import { DescriptionTab } from './DescriptionTab';
import { AttachmentsTab } from './AttachmentsTab';
import { CommentsTab } from './CommentsTab';
import { HistoryTab } from './HistoryTab';

export type TaskDetailTab = 'description' | 'attachments' | 'comments' | 'history';

const TABS: TabItem[] = [
    { id: 'description', label: 'Description' },
    { id: 'attachments', label: 'Attachments' },
    { id: 'comments', label: 'Comments' },
    { id: 'history', label: 'History' },
];

interface TaskDetailModalProps {
    isOpen: boolean;
    taskId: string;
    initialTab: TaskDetailTab;
    onClose: () => void;
    onDeletePermanently?: () => Promise<void>;
}

export const TaskDetailModal: React.FC<TaskDetailModalProps> = ({ isOpen, taskId, initialTab, onClose, onDeletePermanently }) => {
    const [activeTab, setActiveTab] = React.useState<TaskDetailTab>(initialTab);

    React.useEffect(() => {
        if (isOpen) setActiveTab(initialTab);
    }, [isOpen, taskId, initialTab]);

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Task" size="xl">
            <Tabs tabs={TABS} activeTab={activeTab} onChange={(id) => setActiveTab(id as TaskDetailTab)} />
            <div className="p-6">
                {activeTab === 'description' && (
                    <DescriptionTab taskId={taskId} onDeletePermanently={onDeletePermanently} />
                )}
                {activeTab === 'attachments' && <AttachmentsTab taskId={taskId} />}
                {activeTab === 'comments' && <CommentsTab taskId={taskId} />}
                {activeTab === 'history' && <HistoryTab taskId={taskId} />}
            </div>
        </Modal>
    );
};
```

   (`Modal`'s `size` prop only accepts `'sm' | 'md' | 'lg' | 'xl'` per its confirmed interface
   — `xl` → `max-w-4xl`, appropriate for a 4-tab detail view versus the plain form's `lg`.)

### Verification
`docker exec devbridge_frontend npx tsc --noEmit` — will show errors until Tasks 9-11's tab
components exist; acceptable at this point, resolved by Task 11's integration check.

---

## Task 9: `DescriptionTab` — per-field auto-save + assignee dropdown

### Files
- Create: `frontend/src/components/kanban/task-detail/DescriptionTab.tsx`

### Interfaces
- Consumes: `useTasks(projectId)` (`getTask`, `updateTask`), `useTaskAssignees(projectId)` (Task 6), `Input`/`Select`/`RichTextEditor`/`Badge` (`@/components/ui/*`), `UpdateTaskData`/`TaskFormData`/`TagInputData` (`task.types.ts`).
- Produces: one `updateTask(taskId, { <field>: value })` call per field on blur/change — never one combined submit.

### Steps

1. This is the only genuinely new interaction pattern in the feature (per-field save +
   per-field status). Build it as a self-contained component owning its own local form state
   (seeded from the task, same as `task-form.tsx`'s `formData` init) plus one small
   `saveStatus` map keyed by field name:

```tsx
'use client';

import React, { useState, useEffect } from 'react';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useTasks } from '@/hooks/kanban';
import { useTaskAssignees } from '@/hooks/kanban/use-task-assignees';
import { useParams } from 'next/navigation';
import type { TagLevel } from '@/types/kanban';
import { Check, Loader2, AlertCircle, Trash2, X } from 'lucide-react';

type FieldStatus = 'idle' | 'saving' | 'saved' | 'error';

const LEVEL_ORDER: TagLevel[] = ['low', 'medium', 'high'];

interface DescriptionTabProps {
    taskId: string;
    onDeletePermanently?: () => Promise<void>;
}

export const DescriptionTab: React.FC<DescriptionTabProps> = ({ taskId, onDeletePermanently }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask, updateTask } = useTasks(projectId);
    const { assignees, loadAssignees } = useTaskAssignees(projectId);
    const task = getTask(taskId);

    const [title, setTitle] = useState(task?.title ?? '');
    const [description, setDescription] = useState(task?.description ?? '');
    const [htmlDescription, setHtmlDescription] = useState(task?.htmlDescription ?? '');
    const [priority, setPriority] = useState(task?.priority ?? 'medium');
    const [dueDate, setDueDate] = useState(task?.dueDate ?? '');
    const [estimatedHours, setEstimatedHours] = useState(task?.estimatedHours ?? 0);
    const [assigneeId, setAssigneeId] = useState(task?.assigneeId ?? '');
    const [tags, setTags] = useState(task?.tags ?? []);
    const [tagInput, setTagInput] = useState('');
    const [status, setStatus] = useState<Record<string, FieldStatus>>({});
    const [isDeleting, setIsDeleting] = useState(false);

    useEffect(() => { loadAssignees(); }, [loadAssignees]);

    const save = async (field: string, data: Record<string, unknown>) => {
        setStatus((s) => ({ ...s, [field]: 'saving' }));
        try {
            await updateTask(taskId, data);
            setStatus((s) => ({ ...s, [field]: 'saved' }));
            setTimeout(() => setStatus((s) => ({ ...s, [field]: 'idle' })), 1500);
        } catch {
            setStatus((s) => ({ ...s, [field]: 'error' }));
        }
    };

    const Indicator = ({ field }: { field: string }) => {
        const s = status[field] ?? 'idle';
        if (s === 'saving') return <Loader2 className="w-3.5 h-3.5 animate-spin text-gray-400" />;
        if (s === 'saved') return <Check className="w-3.5 h-3.5 text-green-600" />;
        if (s === 'error') return <AlertCircle className="w-3.5 h-3.5 text-red-600" />;
        return null;
    };

    const commitTags = (next: typeof tags) => {
        setTags(next);
        save('tags', { tags: next.map((t) => ({ name: t.name, level: t.level })) });
    };
    const handleAddTag = () => {
        if (!tagInput.trim()) return;
        commitTags([...tags, { id: tagInput, name: tagInput, color: 'blue', level: 'medium' as TagLevel }]);
        setTagInput('');
    };
    const handleRemoveTag = (name: string) => commitTags(tags.filter((t) => t.name !== name));
    const handleCycleTagLevel = (name: string) => commitTags(tags.map((t) =>
        t.name === name ? { ...t, level: LEVEL_ORDER[(LEVEL_ORDER.indexOf(t.level) + 1) % LEVEL_ORDER.length] } : t
    ));

    if (!task) return null;

    return (
        <div className="space-y-6">
            <div className="flex items-center gap-2">
                <div className="flex-1">
                    <Input
                        label="Task Title"
                        value={title}
                        onChange={setTitle}
                        onBlur={() => save('title', { title })}
                        required
                    />
                </div>
                <Indicator field="title" />
            </div>

            <div className="space-y-2">
                <label className="block text-sm font-medium text-gray-700">Description</label>
                <div className="flex items-start gap-2">
                    <div className="flex-1">
                        <RichTextEditor
                            content={htmlDescription}
                            onChange={(html, text) => { setHtmlDescription(html); setDescription(text); }}
                            onBlur={() => save('description', { description, htmlDescription })}
                            minHeight="120px"
                        />
                    </div>
                    <Indicator field="description" />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Select
                            label="Priority"
                            value={priority}
                            onChange={(value) => { setPriority(value); save('priority', { priority: value }); }}
                            options={[
                                { value: 'low', label: 'Low' },
                                { value: 'medium', label: 'Medium' },
                                { value: 'high', label: 'High' },
                                { value: 'urgent', label: 'Urgent' },
                            ]}
                        />
                    </div>
                    <Indicator field="priority" />
                </div>
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Input
                            label="Estimated Hours"
                            type="number"
                            min="0"
                            step="0.5"
                            value={String(estimatedHours)}
                            onChange={(v) => setEstimatedHours(Number(v))}
                            onBlur={() => save('estimatedHours', { estimatedHours })}
                        />
                    </div>
                    <Indicator field="estimatedHours" />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Input
                            label="Due Date"
                            type="date"
                            value={dueDate}
                            onChange={(v) => { setDueDate(v); save('dueDate', { dueDate: v }); }}
                        />
                    </div>
                    <Indicator field="dueDate" />
                </div>
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Select
                            label="Assignee"
                            value={assigneeId}
                            onChange={(value) => { setAssigneeId(value); save('assignee', { assigneeId: value }); }}
                            options={[
                                { value: '', label: 'Unassigned' },
                                ...assignees.map((a) => ({ value: String(a.user_id), label: a.user_name })),
                            ]}
                        />
                    </div>
                    <Indicator field="assignee" />
                </div>
            </div>

            <div className="space-y-2">
                <label className="block text-sm font-medium text-gray-700">Tags</label>
                <div className="flex flex-wrap items-center gap-2 p-2 border rounded-lg border-gray-300 focus-within:ring-blue-500 focus-within:border-blue-500">
                    {tags.map((tag) => (
                        <Badge key={tag.name} variant="secondary" className="gap-1">
                            <button type="button" onClick={() => handleCycleTagLevel(tag.name)}>{tag.level}</button>
                            {tag.name}
                            <button type="button" onClick={() => handleRemoveTag(tag.name)} className="ml-1 hover:text-red-600">
                                <X size={12} />
                            </button>
                        </Badge>
                    ))}
                    <input
                        className="flex-1 min-w-[120px] outline-none py-1"
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleAddTag(); } }}
                        placeholder="Add tag…"
                    />
                </div>
            </div>

            {onDeletePermanently && (
                <div className="pt-4 border-t">
                    <button
                        type="button"
                        onClick={async () => {
                            if (!confirm('Delete this task permanently? It will be removed from every board it appears on.')) return;
                            setIsDeleting(true);
                            try { await onDeletePermanently(); } finally { setIsDeleting(false); }
                        }}
                        disabled={isDeleting}
                        className="flex items-center gap-2 text-sm text-red-600 hover:text-red-700"
                    >
                        <Trash2 size={14} /> {isDeleting ? 'Deleting…' : 'Delete permanently'}
                    </button>
                </div>
            )}
        </div>
    );
};
```

   This deliberately does not reuse `task-form.tsx` by import — `task-form.tsx` is a
   whole-form single-submit component and stays exactly as-is for create-mode (Task 11 keeps
   it wired to `ModalType.TASK_EDIT` unchanged). `DescriptionTab` is edit-mode-only and a
   parallel, not a wrapper, since the save model is fundamentally different (per-field vs.
   whole-form).

   Before finalizing, verify the real prop names of `Select`'s `options` shape and `Input`'s
   `type`/`min`/`step` passthrough against `frontend/src/components/ui/select.tsx` (confirmed
   earlier: `onChange(e.target.value)`, native `<select>` styling) and `input.tsx` (confirmed:
   spreads `...props` onto a native `<input>`, so `type`/`min`/`step`/`onBlur` all pass
   through natively) — both already confirmed compatible with the code above.

### Verification
`docker exec devbridge_frontend npx tsc --noEmit`. Manual: open an existing task's Description
tab, edit the title and tab away (blur) — indicator shows saving→saved, `GET .../history`
shows a `field_changed` row for `title`. Change priority (immediate save, no blur needed).
Add/remove a tag — one `tags` row per commit. Assign a project member — one `assignee` row
with the resolved name.

---

## Task 10: `AttachmentsTab`, `CommentsTab`, `HistoryTab`

### Files
- Create: `frontend/src/components/kanban/task-detail/AttachmentsTab.tsx`
- Create: `frontend/src/components/kanban/task-detail/CommentsTab.tsx`
- Create: `frontend/src/components/kanban/task-detail/HistoryTab.tsx`

### Interfaces
- `AttachmentsTab`/`CommentsTab` are pure pass-through wrappers — no new logic, since
  `AttachmentList` and `CommentSection` already have no modal chrome of their own and already
  take `taskId` as their only required prop.
- `HistoryTab` consumes `useActivityLog(projectId).loadHistory` (Task 6).

### Steps

1. `AttachmentsTab.tsx` — relocates exactly the block currently gated on `taskId` inside
   `task-form.tsx` (lines 264-297: `AttachmentList` + drag-drop upload), reusing
   `useAttachments` the same way `task-form.tsx` does:

```tsx
'use client';

import React, { useRef } from 'react';
import { useParams } from 'next/navigation';
import { useAttachments, useTasks } from '@/hooks/kanban';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useAuth } from '@/contexts/AuthContext';
import { AttachmentList } from '../attachment-list';

interface AttachmentsTabProps {
    taskId: string;
}

export const AttachmentsTab: React.FC<AttachmentsTabProps> = ({ taskId }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask } = useTasks(projectId);
    const currentTask = getTask(taskId);
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const canManageAttachments = hasPermission('tasks:edit', projectId);
    const { uploadAttachments, deleteAttachment, isLoading, error } = useAttachments(projectId);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const handleFiles = async (files: FileList | null) => {
        if (!files || files.length === 0) return;
        await uploadAttachments(taskId, Array.from(files));
    };

    return (
        <div className="space-y-3">
            <AttachmentList
                attachments={currentTask?.attachments ?? []}
                currentUserId={user ? String(user.id) : undefined}
                canManage={canManageAttachments}
                onDelete={(attachmentId) => deleteAttachment(taskId, attachmentId)}
            />
            {error && <p className="text-sm text-red-600">{error}</p>}
            <div
                className="border-2 border-dashed rounded-lg p-4 text-center text-gray-500"
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => { e.preventDefault(); handleFiles(e.dataTransfer.files); }}
            >
                <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    className="hidden"
                    onChange={(e) => handleFiles(e.target.files)}
                />
                <button type="button" onClick={() => fileInputRef.current?.click()} disabled={isLoading}>
                    {isLoading ? 'Uploading…' : 'Drop files here or click to upload (max 10MB each)'}
                </button>
            </div>
        </div>
    );
};
```

   Read `task-form.tsx` lines 264-297 directly before writing this — it is the real, working
   usage pattern this sketch is modeled on. `AttachmentList`'s real interface (confirmed in
   `attachment-list.tsx`) requires `attachments: TaskAttachment[]`, `canManage: boolean`, and
   accepts `currentUserId?: string`; it has no `taskId` prop. `currentTask` comes from
   `useTasks(projectId).getTask(taskId)`, `user` from `useAuth()` (`@/contexts/AuthContext`),
   and `canManageAttachments` from `usePermissions().hasPermission('tasks:edit', projectId)`
   (`@/hooks/auth/use-permissions`) — match `task-form.tsx`'s exact usage of all three before
   finalizing, since this plan's research captured the section's purpose but not its exact
   JSX byte-for-byte. Also verify `useTasks(projectId).getTask(taskId)` actually returns a
   `Task` with a populated `attachments` array (as `task-form.tsx` assumes via
   `currentTask?.attachments`).

2. `CommentsTab.tsx` — trivial wrapper, since `CommentSection` already takes only `taskId`:

```tsx
'use client';

import React from 'react';
import { CommentSection } from '../comment-section';

interface CommentsTabProps {
    taskId: string;
}

export const CommentsTab: React.FC<CommentsTabProps> = ({ taskId }) => <CommentSection taskId={taskId} />;
```

3. `HistoryTab.tsx` — paginated feed, `useState`/`useEffect` load-on-mount plus a "load more"
   button once a full page comes back (no existing pagination UI precedent to copy, per this
   plan's correction #2 above, so this is a direct, minimal implementation):

```tsx
'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
import { useActivityLog } from '@/hooks/kanban/use-activity-log';
import type { ActivityLogEntry } from '@/types/kanban';

const PAGE_SIZE = 20;

const describe = (entry: ActivityLogEntry): string => {
    const who = entry.user?.name ?? 'Someone';
    switch (entry.eventType) {
        case 'field_changed':
            return `${who} changed ${entry.fieldName}: ${entry.oldValue || '—'} → ${entry.newValue || '—'}`;
        case 'moved':
            return `${who} moved the task: ${entry.oldValue} → ${entry.newValue}`;
        case 'comment_added':
            return `${who} commented: “${entry.newValue}”`;
        case 'comment_deleted':
            return `${who} deleted a comment: “${entry.newValue}”`;
        case 'attachment_added':
            return `${who} attached ${entry.newValue}`;
        case 'attachment_deleted':
            return `${who} removed attachment ${entry.newValue}`;
        default:
            return `${who} made a change`;
    }
};

interface HistoryTabProps {
    taskId: string;
}

export const HistoryTab: React.FC<HistoryTabProps> = ({ taskId }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { loadHistory, isLoading, error } = useActivityLog(projectId);
    const [entries, setEntries] = useState<ActivityLogEntry[]>([]);
    const [hasMore, setHasMore] = useState(false);

    const fetchPage = useCallback(async (offset: number) => {
        const page = await loadHistory(taskId, { limit: PAGE_SIZE, offset });
        setEntries((prev) => (offset === 0 ? page : [...prev, ...page]));
        setHasMore(page.length === PAGE_SIZE);
    }, [loadHistory, taskId]);

    useEffect(() => { fetchPage(0); }, [fetchPage]);

    return (
        <div className="space-y-3">
            {entries.map((entry) => (
                <div key={entry.id} className="text-sm border-b pb-2">
                    <p className="text-gray-800">{describe(entry)}</p>
                    <p className="text-xs text-gray-400">{new Date(entry.createdAt).toLocaleString()}</p>
                </div>
            ))}
            {error && <p className="text-sm text-red-600">{error}</p>}
            {hasMore && (
                <button
                    type="button"
                    onClick={() => fetchPage(entries.length)}
                    disabled={isLoading}
                    className="text-sm text-blue-600 hover:text-blue-700"
                >
                    {isLoading ? 'Loading…' : 'Load more'}
                </button>
            )}
        </div>
    );
};
```

### Verification
`docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 11: Wire `TaskDetailModal` into `kanban-board.tsx`

### Files
- Modify: `frontend/src/types/kanban/kanban.types.ts` (lines 52-58)
- Modify: `frontend/src/components/kanban/kanban-board.tsx` (lines 30-34, 42-49, 151-187)

### Steps

1. In `kanban.types.ts`, replace `COMMENTS` with `TASK_DETAIL` (confirmed via repo-wide grep
   that `ModalType`/`activeModal` are referenced only inside `kanban-board.tsx`, so this is a
   safe, contained rename — no other file references `ModalType.COMMENTS`):

```ts
export enum ModalType {
    TASK_EDIT = 'task_edit',
    TASK_DETAIL = 'task_detail',
    TIME_LOG = 'time_log',
    COLUMN_SETTINGS = 'column_settings',
    ADD_TO_BOARD = 'add_to_board',
}
```

2. In `kanban-board.tsx`, extend `activeModal`'s shape (lines 30-34) with `initialTab`, and
   change `handleEditTask`/`handleOpenComments` (lines 42-49) to both route through
   `TASK_DETAIL`:

```tsx
import { TaskDetailModal, TaskDetailTab } from './task-detail/TaskDetailModal';
// ...
    const [activeModal, setActiveModal] = React.useState<{
        type: ModalType | null;
        taskId?: string;
        columnId?: string;
        initialTab?: TaskDetailTab;
    }>({ type: null });

    // ...
    const handleEditTask = (taskId: string) => {
        if (!permissions.canEditTasks) return;
        setActiveModal({ type: ModalType.TASK_DETAIL, taskId, initialTab: 'description' });
    };

    const handleOpenComments = (taskId: string) => {
        setActiveModal({ type: ModalType.TASK_DETAIL, taskId, initialTab: 'comments' });
    };
```

3. Replace the two modal blocks currently at lines 151-187 (the `TASK_EDIT` modal's edit-mode
   branch and the whole `COMMENTS` modal). `TASK_EDIT` stays for create-only (drop the
   `taskId`/`onDeletePermanently` branch — those only ever fire for edit, which no longer
   routes here), and one new `TaskDetailModal` instance replaces the `COMMENTS` block:

```tsx
            <Modal
                isOpen={activeModal.type === ModalType.TASK_EDIT}
                onClose={closeModal}
                title="Create Task"
                size="lg"
            >
                <TaskForm
                    columnId={activeModal.columnId}
                    onSubmit={async (data) => {
                        if (activeModal.columnId) {
                            await createTask({ ...data, columnId: activeModal.columnId, boardId });
                        }
                        closeModal();
                    }}
                    onCancel={closeModal}
                />
            </Modal>

            <TaskDetailModal
                isOpen={activeModal.type === ModalType.TASK_DETAIL}
                taskId={activeModal.taskId ?? ''}
                initialTab={activeModal.initialTab ?? 'description'}
                onClose={closeModal}
                onDeletePermanently={activeModal.taskId ? async () => {
                    await deleteTask(activeModal.taskId!);
                    closeModal();
                } : undefined}
            />
```

   (The `TIME_LOG`, `COLUMN_SETTINGS`, `ADD_TO_BOARD` modal blocks below this, at lines
   188-223, are untouched — time-tracking stays exactly as it is today, out of this feature's
   scope per the spec's non-goals.)

### Verification
`docker exec devbridge_frontend npx tsc --noEmit` — zero errors across the whole feature now
that every consumer exists. Manual: reload the kanban board, click a task's edit icon → opens
on Description tab; click its comment icon → opens on Comments tab, same modal; switch tabs
freely; click "+ Add task" on a column → still opens the old single-panel create form, unaffected.

---

## Task 12: Final integration verification

### Steps
1. `docker exec devbridge_backend go build ./... && go vet ./...`
2. `docker exec devbridge_frontend npx tsc --noEmit`
3. Full manual walkthrough on a disposable test project/board/task (create it, exercise every
   tab and every field's auto-save, move it between columns, add/delete a comment, add/delete
   an attachment, assign/reassign it), confirming the History tab's timeline matches every
   action in the correct order with correct human-readable text, then delete the disposable
   project/task afterward.
4. Report exact commands run, pass/fail, and any pre-existing unrelated errors separated out
   (do not infer full-suite success from these targeted checks).

---

## Execution

Plan complete and saved to `docs/superpowers/plans/2026-09-04-task-detail-history.md`.

Two ways to execute it:

1. **Subagent-Driven Development** — dispatch each task to a fresh subagent one at a time,
   review its diff before moving to the next, using the SDD ledger pattern already used for
   the task-attachments feature this session.
2. **Inline Execution** — implement the tasks directly in this conversation, task by task,
   with the same per-task container verification gates as written above.

Which approach do you want?
