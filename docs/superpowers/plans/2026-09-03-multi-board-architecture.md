# Multi-Board Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce `Board` as a first-class entity (many per project), decouple a task's shared data from its per-board placement (column/position), and update the frontend so a task can be created, moved, added to another board, removed from one board (surviving on others), or deleted permanently everywhere.

**Architecture:** One migration reshapes the schema (`boards`, `task_placements` join table, drops `tasks.column_id/position` and `kanban_columns.project_id`). Backend DTOs keep their existing `ColumnID`/`Position` shape but source those values from an optional `*models.TaskPlacement` parameter, so callers that don't know a board (project-scoped `GetTasks`/`GetTask`/`UpdateTask`) get placement-independent DTOs, while board-scoped callers (`CreateTask`, `MoveTask`, `PlaceTask`) get real values. Frontend gains a boards-list page per project, board-scoped routes/services/hooks, and task-card actions for cross-board placement.

**Tech Stack:** Go + Fiber + GORM + PostgreSQL (backend), Next.js + React + Zustand + TypeScript (frontend), Playwright (E2E), all tooling run via `docker exec devbridge_backend` / `docker exec devbridge_frontend`.

**Spec:** `docs/superpowers/specs/2026-09-03-multi-board-architecture-design.md`

## Global Constraints

- No new dependencies, infra, queues, or environment variables — extend existing patterns only.
- Successful kanban endpoints return raw entity/array JSON (no wrapper). Error responses and delete-confirmation successes use `fiber.Map{"success": bool, "message": string}`.
- IDs are serialized as strings via `models.IDToStr`/parsed via `models.StrToID`.
- Board CRUD reuses the existing `kanban.manage_columns` permission — no new permissions are created.
- Migrations auto-run via `RunMigrations` (`backend/internal/database/migrate.go`) — the new `000012` migration needs no special-casing; it runs through the standard `golang-migrate` path like every migration after `000006`.
- All backend/frontend tooling (`go build`, `npx tsc`, migrations, curl) runs only inside `devbridge_backend` / `devbridge_frontend` containers — never on the host.
- No Go test files exist in this repo (`*_test.go` — none). Backend verification is `go build ./...` plus manual `curl` against the running dev container.
- No frontend test framework is configured (`package.json` scripts are only `dev`/`build`/`start`). Frontend verification is `npx tsc --noEmit` plus manual dev-server click-through, culminating in a Playwright script (Task 9) run the same way `column_settings_check2.js` was run this session.
- Comments and time-entry routes stay at their current project/task-scoped paths (`/projects/:id/tasks/:taskId/comments`, `/projects/:id/tasks/:taskId/time-entries`, `/projects/:id/comments/:commentId`, `/projects/:id/time-entries/:entryId`) — they are task-scoped shared data with no dependency on which board is being viewed, so `task_comment_handler.go` and `task_time_entry_handler.go` are not touched by this plan.

---

## Backend

### Task 1: Migration `000012` — boards and task placements

**Files:**
- Create: `backend/migrations/000012_create_boards_and_placements.up.sql`
- Create: `backend/migrations/000012_create_boards_and_placements.down.sql`

**Interfaces:**
- Consumes: existing tables `projects`, `kanban_columns`, `tasks` (see `backend/migrations/000011_create_kanban_tables.up.sql` for the schema this builds on).
- Produces: `boards(id, project_id, name, position, created_at, updated_at)`, `kanban_columns.board_id` (replacing `kanban_columns.project_id`), `task_placements(id, task_id, board_id, column_id, position, created_at, updated_at)` with a unique constraint on `(task_id, board_id)`, `tasks` with `column_id`/`position` dropped. These are consumed by Task 2's Go models.

- [ ] **Step 1: Write the up migration**

```sql
-- backend/migrations/000012_create_boards_and_placements.up.sql

CREATE TABLE boards (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_boards_project_id ON boards(project_id);

INSERT INTO boards (project_id, name, position, created_at, updated_at)
SELECT DISTINCT project_id, 'Main Board', 0, NOW(), NOW()
FROM kanban_columns;

ALTER TABLE kanban_columns ADD COLUMN board_id INTEGER;

UPDATE kanban_columns kc
SET board_id = b.id
FROM boards b
WHERE b.project_id = kc.project_id;

ALTER TABLE kanban_columns ALTER COLUMN board_id SET NOT NULL;
ALTER TABLE kanban_columns ADD CONSTRAINT kanban_columns_board_id_fkey FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE;
CREATE INDEX idx_kanban_columns_board_id ON kanban_columns(board_id);
ALTER TABLE kanban_columns DROP COLUMN project_id;

CREATE TABLE task_placements (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id INTEGER NOT NULL REFERENCES kanban_columns(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(task_id, board_id)
);

CREATE INDEX idx_task_placements_task_id ON task_placements(task_id);
CREATE INDEX idx_task_placements_board_id ON task_placements(board_id);
CREATE INDEX idx_task_placements_column_id ON task_placements(column_id);

INSERT INTO task_placements (task_id, board_id, column_id, position, created_at, updated_at)
SELECT t.id, kc.board_id, t.column_id, t.position, NOW(), NOW()
FROM tasks t
JOIN kanban_columns kc ON kc.id = t.column_id;

ALTER TABLE tasks DROP COLUMN column_id;
ALTER TABLE tasks DROP COLUMN position;
```

- [ ] **Step 2: Write the down migration**

```sql
-- backend/migrations/000012_create_boards_and_placements.down.sql

ALTER TABLE tasks ADD COLUMN column_id INTEGER;
ALTER TABLE tasks ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

UPDATE tasks t
SET column_id = fp.column_id, position = fp.position
FROM (
    SELECT DISTINCT ON (task_id) task_id, column_id, position
    FROM task_placements
    ORDER BY task_id, id ASC
) fp
WHERE fp.task_id = t.id;

DELETE FROM tasks WHERE column_id IS NULL;

ALTER TABLE tasks ALTER COLUMN column_id SET NOT NULL;
ALTER TABLE tasks ADD CONSTRAINT tasks_column_id_fkey FOREIGN KEY (column_id) REFERENCES kanban_columns(id) ON DELETE CASCADE;
CREATE INDEX idx_tasks_column_id ON tasks(column_id);

DROP TABLE IF EXISTS task_placements;

ALTER TABLE kanban_columns ADD COLUMN project_id INTEGER;

UPDATE kanban_columns kc
SET project_id = b.project_id
FROM boards b
WHERE b.id = kc.board_id;

ALTER TABLE kanban_columns ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE kanban_columns ADD CONSTRAINT kanban_columns_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
CREATE INDEX idx_kanban_columns_project_id ON kanban_columns(project_id);
ALTER TABLE kanban_columns DROP COLUMN board_id;

DROP TABLE IF EXISTS boards;
```

Note: a task with zero placements has no matching row in the `fp` subquery, so `column_id` stays `NULL` and is deleted by the `DELETE FROM tasks WHERE column_id IS NULL` line — this is the documented lossy-simplification the spec calls out ("acceptable simplification for local dev data... documented as a limitation").

- [ ] **Step 3: Run the migration and verify the schema**

```bash
docker exec devbridge_backend go run ./cmd/server &
sleep 3
docker exec devbridge_backend pkill -f 'go run ./cmd/server' || true
docker exec devbridge_postgres psql -U devbridge -d devbridge -c "\d boards"
docker exec devbridge_postgres psql -U devbridge -d devbridge -c "\d task_placements"
docker exec devbridge_postgres psql -U devbridge -d devbridge -c "\d kanban_columns"
docker exec devbridge_postgres psql -U devbridge -d devbridge -c "\d tasks"
```

(Adjust the postgres container name/credentials to match `docker-compose.yml` if they differ — check with `docker ps` and the compose file before running.)

Expected: `boards` and `task_placements` exist with the columns above; `kanban_columns` has `board_id` and no `project_id`; `tasks` has neither `column_id` nor `position`.

- [ ] **Step 4: Verify data was preserved**

```bash
docker exec devbridge_postgres psql -U devbridge -d devbridge -c "SELECT count(*) FROM boards;"
docker exec devbridge_postgres psql -U devbridge -d devbridge -c "SELECT count(*) FROM task_placements;"
```

Expected: one board per project that previously had columns; one placement per pre-existing task.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/000012_create_boards_and_placements.up.sql backend/migrations/000012_create_boards_and_placements.down.sql
git commit -m "feat: add boards and task_placements migration"
```

---

### Task 2: Backend data model, DTOs, handlers, and routes

This is one task, not several, because Go requires the whole program to compile together: removing `Task.ColumnID`/`Task.Position` (required by the spec) immediately breaks every file that references them, so models, the DTO builder, both kanban handlers, the new board handler, and the routes file must land together for `go build ./...` to succeed. Steps below are still bite-sized (one function/type per step) — only the final build/curl verification is deferred to the end.

**Files:**
- Modify: `backend/internal/models/kanban.go`
- Modify: `backend/internal/handlers/kanban_dto_builder.go`
- Modify: `backend/internal/handlers/kanban_handler.go`
- Create: `backend/internal/handlers/board_handler.go`
- Modify: `backend/internal/handlers/task_handler.go`
- Modify: `backend/internal/routes/kanban_routes.go`
- Test: none (no `*_test.go` files exist) — verified via `go build ./...` and `curl`.

**Interfaces:**
- Consumes: `boards`/`task_placements`/`kanban_columns.board_id` schema from Task 1.
- Produces (for Task 3+ frontend tasks to match exactly):
  - `GET/PUT /projects/:id/boards` and `/projects/:id/boards/:boardId` — Board entity CRUD, returns `BoardDTO{id, projectId, name, position, createdAt, updatedAt}` (raw object/array, no wrapper) or `fiber.Map{"success": true/false, "message": ...}` for delete.
  - `GET/PUT /projects/:id/boards/:boardId/kanban` — board content, returns `KanbanBoardDTO{id, projectId, name, columns, settings, createdAt, updatedAt}`.
  - `POST/PUT/DELETE /projects/:id/boards/:boardId/kanban/columns...` — unchanged shapes, now board-scoped.
  - `PUT /projects/:id/boards/:boardId/tasks/:taskId/move` — body `{columnId, position}`, returns `TaskDTO`.
  - `POST /projects/:id/boards/:boardId/tasks/:taskId/place` — body `{columnId}`, returns `TaskDTO` (201).
  - `DELETE /projects/:id/boards/:boardId/tasks/:taskId` — removes this board's placement; returns `fiber.Map{"success": true, "message": "Task removed from board successfully"}`.
  - `POST /projects/:id/tasks` — body now requires `boardId` in addition to `columnId`.
  - `GET /projects/:id/tasks`, `GET/PUT /projects/:id/tasks/:taskId`, `DELETE /projects/:id/tasks/:taskId` — unchanged paths; `DELETE` still deletes everywhere.
  - `models.TaskDTO.ColumnID`/`.Position` are `""`/`0` when the DTO was built without a placement (project-scoped contexts), real values otherwise.

- [ ] **Step 1: Add `Board` and `TaskPlacement` models**

In `backend/internal/models/kanban.go`, add near the top (after the `KanbanColumn` struct):

```go
type Board struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"not null;index" json:"projectId"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Board) TableName() string { return "boards" }

type TaskPlacement struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"not null;index" json:"taskId"`
	BoardID   uint      `gorm:"not null;index" json:"boardId"`
	ColumnID  uint      `gorm:"not null;index" json:"columnId"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TaskPlacement) TableName() string { return "task_placements" }
```

- [ ] **Step 2: Change `KanbanColumn.ProjectID` to `BoardID`**

Find the `KanbanColumn` struct field `ProjectID uint` and rename it (and its gorm tag) to:

```go
	BoardID uint `gorm:"not null;index" json:"boardId"`
```

- [ ] **Step 3: Remove `ColumnID`/`Position` from `Task`**

In the `Task` struct, delete the `ColumnID uint` and `Position int` fields entirely (everything else — `ProjectID`, `Title`, `Description`, `HTMLDescription`, `Priority`, `Status`, `AssigneeID`, `EstimatedHours`, `Tags`, `DueDate`, `IsArchived`, `CreatedBy`, `UpdatedBy`, `CreatedAt`, `UpdatedAt` — stays unchanged).

- [ ] **Step 4: Add `Name` to `KanbanBoardDTO`, add `BoardDTO`**

In `KanbanBoardDTO`, add a field:

```go
	Name string `json:"name"`
```

Add a new struct below it:

```go
type BoardDTO struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```

- [ ] **Step 5: Add `BoardID` to `CreateTaskRequest`, add new request types**

In `CreateTaskRequest`, add:

```go
	BoardID string `json:"boardId" validate:"required"`
```

Add new structs (near `CreateColumnRequest`/`UpdateColumnRequest`):

```go
type CreateBoardRequest struct {
	Name     string `json:"name" validate:"required"`
	Position int    `json:"position"`
}

type UpdateBoardRequest struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}

type PlaceTaskRequest struct {
	ColumnID string `json:"columnId" validate:"required"`
}
```

- [ ] **Step 6: Rewrite `buildTaskDTO` to source column/position from an optional placement**

In `backend/internal/handlers/kanban_dto_builder.go`, change the `buildTaskDTO` signature and body:

```go
func buildTaskDTO(t models.Task, placement *models.TaskPlacement, comments []models.TaskComment, entries []models.TaskTimeEntry, users map[uint]models.User) models.TaskDTO {
	columnID := ""
	position := 0
	if placement != nil {
		columnID = models.IDToStr(placement.ColumnID)
		position = placement.Position
	}

	dto := models.TaskDTO{
		ID:              models.IDToStr(t.ID),
		ProjectID:       models.IDToStr(t.ProjectID),
		ColumnID:        columnID,
		Title:           t.Title,
		Description:     t.Description,
		HTMLDescription: t.HTMLDescription,
		Priority:        t.Priority,
		Status:          t.Status,
		EstimatedHours:  t.EstimatedHours,
		Tags:            tagsWithColors(models.TagsFromJSON(t.Tags)),
		Position:        position,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
		CreatedBy:       models.IDToStr(t.CreatedBy),
		UpdatedBy:       models.IDToStr(t.UpdatedBy),
	}

	if t.DueDate != nil {
		dto.DueDate = models.FormatDate(*t.DueDate)
	}
	if t.AssigneeID != nil {
		dto.AssigneeID = models.IDToStr(*t.AssigneeID)
		if user, ok := users[*t.AssigneeID]; ok {
			dto.Assignee = assigneeDTO(user)
		}
	}

	dto.Comments = make([]models.TaskCommentDTO, 0, len(comments))
	for _, c := range comments {
		dto.Comments = append(dto.Comments, buildCommentDTO(c, users))
	}

	dto.TimeEntries = make([]models.TaskTimeEntryDTO, 0, len(entries))
	for _, e := range entries {
		dto.TimeEntries = append(dto.TimeEntries, buildTimeEntryDTO(e, users))
	}

	loggedHours := 0.0
	for _, e := range entries {
		loggedHours += e.Hours
	}
	dto.LoggedHours = loggedHours

	return dto
}
```

Note: this reproduces the field list your read of the original `buildTaskDTO` established (ID, ProjectID, Title, Description, HTMLDescription, Priority, Status, EstimatedHours, Tags via `tagsWithColors`/`TagsFromJSON`, CreatedAt/UpdatedAt/CreatedBy/UpdatedBy, DueDate via `FormatDate`, AssigneeID/Assignee via `assigneeDTO`, Comments via `buildCommentDTO`, TimeEntries via `buildTimeEntryDTO`, LoggedHours summed from entries) — only `ColumnID`/`Position` change from direct `t.ColumnID`/`t.Position` reads to the placement-conditional logic above. If any helper name here (`tagsWithColors`, `assigneeDTO`, `buildCommentDTO`, `buildTimeEntryDTO`) doesn't match the current file verbatim, keep the existing helper calls from the current `buildTaskDTO` body and only change the `ColumnID`/`Position` lines as shown.

- [ ] **Step 7: Update `loadTaskDTOs` to call `buildTaskDTO` with a nil placement**

Find every call site of `buildTaskDTO(t, ...)` inside `loadTaskDTOs(projectID uint)` and insert `nil` as the second argument: `buildTaskDTO(t, nil, commentsByTask[t.ID], entriesByTask[t.ID], users)`. This function stays project-scoped and placement-independent, matching `GET /projects/:id/tasks`.

- [ ] **Step 8: Add placement parameter to `loadSingleTaskDTO`**

Change the signature to:

```go
func loadSingleTaskDTO(task models.Task, placement *models.TaskPlacement) models.TaskDTO {
```

and update its internal call to `buildTaskDTO(task, placement, comments, entries, users)` (keep the rest of the function — loading comments/entries/users for the single task — unchanged).

- [ ] **Step 9: Add `loadBoardTaskDTOs` for board-scoped task loading**

Add this new function to `kanban_dto_builder.go`:

```go
// loadBoardTaskDTOs loads every task placed on a board, using each task's
// board-specific placement for its column/position.
func loadBoardTaskDTOs(boardID uint) ([]models.TaskDTO, error) {
	var placements []models.TaskPlacement
	if err := database.GetDB().Where("board_id = ?", boardID).Find(&placements).Error; err != nil {
		return nil, err
	}
	if len(placements) == 0 {
		return []models.TaskDTO{}, nil
	}

	placementByTask := make(map[uint]models.TaskPlacement, len(placements))
	taskIDs := make([]uint, 0, len(placements))
	for _, p := range placements {
		placementByTask[p.TaskID] = p
		taskIDs = append(taskIDs, p.TaskID)
	}

	var tasks []models.Task
	if err := database.GetDB().Where("id IN ? AND is_archived = false", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}

	userIDs := make([]uint, 0, len(tasks)*2)
	rowIDs := make([]uint, len(tasks))
	for i, t := range tasks {
		rowIDs[i] = t.ID
		userIDs = append(userIDs, t.CreatedBy, t.UpdatedBy)
		if t.AssigneeID != nil {
			userIDs = append(userIDs, *t.AssigneeID)
		}
	}

	var comments []models.TaskComment
	database.GetDB().Where("task_id IN ?", rowIDs).Order("created_at ASC").Find(&comments)

	var entries []models.TaskTimeEntry
	database.GetDB().Where("task_id IN ?", rowIDs).Order("date DESC").Find(&entries)

	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	users := loadUsersByIDs(userIDs)

	commentsByTask := make(map[uint][]models.TaskComment)
	for _, c := range comments {
		commentsByTask[c.TaskID] = append(commentsByTask[c.TaskID], c)
	}
	entriesByTask := make(map[uint][]models.TaskTimeEntry)
	for _, e := range entries {
		entriesByTask[e.TaskID] = append(entriesByTask[e.TaskID], e)
	}

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		p := placementByTask[t.ID]
		dtos = append(dtos, buildTaskDTO(t, &p, commentsByTask[t.ID], entriesByTask[t.ID], users))
	}
	return dtos, nil
}
```

- [ ] **Step 10: Rewrite `kanban_handler.go`'s `ensureColumns` to be board-scoped**

Replace the existing `ensureColumns(projectID uint)` with:

```go
func ensureColumns(boardID uint) ([]models.KanbanColumn, error) {
	var columns []models.KanbanColumn
	if err := database.GetDB().Where("board_id = ?", boardID).Order("position ASC").Find(&columns).Error; err != nil {
		return nil, err
	}
	if len(columns) > 0 {
		return columns, nil
	}

	seeded := make([]models.KanbanColumn, 0, len(defaultColumnSeed))
	for i, c := range defaultColumnSeed {
		col := models.KanbanColumn{
			BoardID:  boardID,
			Title:    c.Title,
			Color:    c.Color,
			Position: i,
		}
		if i == 1 {
			col.MaxTasks = intPtr(5)
		}
		seeded = append(seeded, col)
	}
	if err := database.GetDB().Create(&seeded).Error; err != nil {
		return nil, err
	}
	return seeded, nil
}
```

(Keep `defaultColumnSeed` and `intPtr` exactly as they are — only `ensureColumns` changes.)

- [ ] **Step 11: Rewrite `columnDTO`'s task filter to match string IDs (unchanged logic, confirm it still compiles)**

`columnDTO(c models.KanbanColumn, tasks []models.TaskDTO) models.KanbanColumnDTO` already filters by `t.ColumnID == models.IDToStr(c.ID)` — this still works unchanged since `TaskDTO.ColumnID` is still a string; no edit needed here, just confirm it compiles after Steps 1-10.

- [ ] **Step 12: Rewrite `GetBoard` to be board-scoped**

Replace the `GetBoard` method body with:

```go
func (h *KanbanHandler) GetBoard(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var board models.Board
	if err := database.GetDB().First(&board, boardID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Board not found"})
	}

	columns, err := ensureColumns(uint(boardID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading board"})
	}

	taskDTOs, err := loadBoardTaskDTOs(uint(boardID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading tasks"})
	}

	columnDTOs := make([]models.KanbanColumnDTO, 0, len(columns))
	for _, col := range columns {
		columnDTOs = append(columnDTOs, columnDTO(col, taskDTOs))
	}

	return c.JSON(models.KanbanBoardDTO{
		ID:        models.IDToStr(board.ID),
		ProjectID: models.IDToStr(board.ProjectID),
		Name:      board.Name,
		Columns:   columnDTOs,
		Settings:  models.DefaultKanbanSettings(),
		CreatedAt: board.CreatedAt,
		UpdatedAt: board.UpdatedAt,
	})
}
```

`UpdateBoard` (board content) stays exactly as `return h.GetBoard(c)` — no change needed, since it already delegates and `GetBoard` now reads `boardId` from `c.Params`.

- [ ] **Step 13: Rewrite `CreateColumn` to read `boardId` from params**

```go
func (h *KanbanHandler) CreateColumn(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var req models.CreateColumnRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title is required"})
	}
	if req.Color == "" {
		req.Color = "bg-gray-500"
	}

	column := models.KanbanColumn{
		BoardID:  uint(boardID),
		Title:    req.Title,
		Color:    req.Color,
		Position: req.Position,
		MaxTasks: req.MaxTasks,
	}
	if err := database.GetDB().Create(&column).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating column"})
	}

	return c.Status(201).JSON(columnDTO(column, []models.TaskDTO{}))
}
```

If `CreateColumnRequest` doesn't have exactly `Title`, `Color`, `Position`, `MaxTasks` fields, keep the field names as they exist in `models.kanban.go` and adjust only the `BoardID: uint(boardID)` line (was `ProjectID: uint(projectID)`).

- [ ] **Step 14: Update `UpdateColumn` to reload via `loadBoardTaskDTOs`**

Find the line(s) after saving the column update that call `loadTaskDTOs(column.ProjectID)` (or similar) to rebuild the DTO response, and change to:

```go
	taskDTOs, _ := loadBoardTaskDTOs(column.BoardID)
```

(keep the rest of `UpdateColumn` — parsing the request, applying updates, saving, building the `columnDTO` response — unchanged).

- [ ] **Step 15: Update `DeleteColumn` with the same last-placement-aware cascade delete as `DeleteBoard`; confirm `ReorderColumns` needs no changes**

Before the migration, `tasks.column_id` had `ON DELETE CASCADE`, so deleting a column cascaded straight through to delete its tasks (and their comments/time entries) at the DB level — `DeleteColumn` didn't need any application-level cleanup. After this migration, `task_placements.column_id` still cascades on column delete, but that only removes the *placement* rows; a task whose deleted placement was its last one is no longer reachable by any DB-level cascade and would be silently orphaned (kept in `tasks` forever, invisible on every board) unless `DeleteColumn` cleans it up explicitly — which contradicts the spec's requirement that "a task is only actually destroyed if the column being deleted holds its last placement." Apply the identical pattern used in `DeleteBoard` (Step 16 below), scoped to the column instead of the board:

```go
func (h *KanbanHandler) DeleteColumn(c *fiber.Ctx) error {
	columnID, err := strconv.Atoi(c.Params("columnId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	var column models.KanbanColumn
	if err := database.GetDB().First(&column, columnID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Column not found"})
	}

	db := database.GetDB()

	var affectedTaskIDs []uint
	db.Model(&models.TaskPlacement{}).Where("column_id = ?", columnID).Pluck("task_id", &affectedTaskIDs)

	if err := db.Delete(&column).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting column"})
	}

	if len(affectedTaskIDs) > 0 {
		var stillPlaced []uint
		db.Model(&models.TaskPlacement{}).Where("task_id IN ?", affectedTaskIDs).Pluck("task_id", &stillPlaced)

		stillPlacedSet := make(map[uint]bool, len(stillPlaced))
		for _, id := range stillPlaced {
			stillPlacedSet[id] = true
		}

		orphaned := make([]uint, 0)
		for _, id := range affectedTaskIDs {
			if !stillPlacedSet[id] {
				orphaned = append(orphaned, id)
			}
		}
		if len(orphaned) > 0 {
			db.Delete(&models.Task{}, orphaned)
		}
	}

	return c.JSON(fiber.Map{"success": true, "message": "Column deleted successfully"})
}
```

`ReorderColumns` genuinely needs no change — it only ever updates `kanban_columns.position` for column IDs it's given, never touches placements or task existence, so it's correct as-is regardless of board/placement scoping.

- [ ] **Step 16: Create `board_handler.go`**

```go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type BoardHandler struct{}

func NewBoardHandler() *BoardHandler {
	return &BoardHandler{}
}

func boardDTO(b models.Board) models.BoardDTO {
	return models.BoardDTO{
		ID:        models.IDToStr(b.ID),
		ProjectID: models.IDToStr(b.ProjectID),
		Name:      b.Name,
		Position:  b.Position,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

// ListBoards - GET /api/v1/projects/:id/boards
func (h *BoardHandler) ListBoards(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var boards []models.Board
	if err := database.GetDB().Where("project_id = ?", projectID).Order("position ASC").Find(&boards).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading boards"})
	}

	dtos := make([]models.BoardDTO, 0, len(boards))
	for _, b := range boards {
		dtos = append(dtos, boardDTO(b))
	}
	return c.JSON(dtos)
}

// CreateBoard - POST /api/v1/projects/:id/boards
// Seeds the same default columns used for a project's first board.
func (h *BoardHandler) CreateBoard(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.CreateBoardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Name is required"})
	}

	board := models.Board{
		ProjectID: uint(projectID),
		Name:      req.Name,
		Position:  req.Position,
	}
	if err := database.GetDB().Create(&board).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating board"})
	}

	if _, err := ensureColumns(board.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error seeding board columns"})
	}

	return c.Status(201).JSON(boardDTO(board))
}

// UpdateBoard - PUT /api/v1/projects/:id/boards/:boardId
func (h *BoardHandler) UpdateBoard(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var board models.Board
	if err := database.GetDB().First(&board, boardID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Board not found"})
	}

	var req models.UpdateBoardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Name != nil {
		board.Name = *req.Name
	}
	if req.Position != nil {
		board.Position = *req.Position
	}

	if err := database.GetDB().Save(&board).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating board"})
	}

	return c.JSON(boardDTO(board))
}

// DeleteBoard - DELETE /api/v1/projects/:id/boards/:boardId
// Cascades (via FK) to the board's columns and their task_placements. Any
// task whose deleted placement was its last one is hard-deleted here.
func (h *BoardHandler) DeleteBoard(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var board models.Board
	if err := database.GetDB().First(&board, boardID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Board not found"})
	}

	db := database.GetDB()

	var affectedTaskIDs []uint
	db.Model(&models.TaskPlacement{}).Where("board_id = ?", boardID).Pluck("task_id", &affectedTaskIDs)

	if err := db.Delete(&board).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting board"})
	}

	if len(affectedTaskIDs) > 0 {
		var stillPlaced []uint
		db.Model(&models.TaskPlacement{}).Where("task_id IN ?", affectedTaskIDs).Pluck("task_id", &stillPlaced)

		stillPlacedSet := make(map[uint]bool, len(stillPlaced))
		for _, id := range stillPlaced {
			stillPlacedSet[id] = true
		}

		orphaned := make([]uint, 0)
		for _, id := range affectedTaskIDs {
			if !stillPlacedSet[id] {
				orphaned = append(orphaned, id)
			}
		}
		if len(orphaned) > 0 {
			db.Delete(&models.Task{}, orphaned)
		}
	}

	return c.JSON(fiber.Map{"success": true, "message": "Board deleted successfully"})
}
```

- [ ] **Step 17: Update `task_handler.go`'s `GetTask` and `UpdateTask` to pass `nil` placement**

Find both call sites of `loadSingleTaskDTO(task)` in `GetTask` and `UpdateTask` and change them to `loadSingleTaskDTO(task, nil)`.

- [ ] **Step 18: Rewrite `CreateTask`**

```go
func (h *TaskHandler) CreateTask(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title is required"})
	}

	boardID, err := models.StrToID(req.BoardID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	columnID, err := models.StrToID(req.ColumnID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	var estimatedHours float64
	if req.EstimatedHours != nil {
		estimatedHours = *req.EstimatedHours
	}

	dueDate, err := models.ParseDate(req.DueDate)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid due date"})
	}

	var assigneeID *uint
	if req.AssigneeID != "" {
		id, err := models.StrToID(req.AssigneeID)
		if err == nil {
			assigneeID = &id
		}
	}

	userID := currentUserID(c)

	task := models.Task{
		ProjectID:       uint(projectID),
		Title:           req.Title,
		Description:     req.Description,
		HTMLDescription: req.HTMLDescription,
		Priority:        priority,
		Status:          "todo",
		AssigneeID:      assigneeID,
		EstimatedHours:  estimatedHours,
		Tags:            models.TagsToJSON(req.Tags),
		DueDate:         dueDate,
		CreatedBy:       userID,
		UpdatedBy:       userID,
	}
	if err := database.GetDB().Create(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating task"})
	}

	var maxPosition struct{ Max int }
	database.GetDB().Model(&models.TaskPlacement{}).
		Select("COALESCE(MAX(position), -1) as max").
		Where("column_id = ?", columnID).
		Scan(&maxPosition)

	placement := models.TaskPlacement{
		TaskID:   task.ID,
		BoardID:  boardID,
		ColumnID: columnID,
		Position: maxPosition.Max + 1,
	}
	if err := database.GetDB().Create(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error placing task"})
	}

	return c.Status(201).JSON(loadSingleTaskDTO(task, &placement))
}
```

If `CreateTaskRequest.AssigneeID` is not a plain `string` in the current file, adjust the assignee-parsing block to match its actual type — otherwise keep every field name above as-is (matches the fields read this session: `Title, Description, HTMLDescription, Priority, ColumnID, AssigneeID, EstimatedHours, Tags, DueDate`, plus the new `BoardID`).

- [ ] **Step 19: Confirm `DeleteTask` needs no change**

`DeleteTask` already does `database.GetDB().Delete(&models.Task{}, taskID)` — an unconditional hard delete matching the spec's "delete permanently... regardless of how many placements" requirement. Leave it exactly as-is (comments/time entries cascade via existing FKs; task_placements also cascades via its FK to `tasks`).

- [ ] **Step 20: Rewrite `MoveTask` to be board-scoped and placement-based**

```go
func (h *TaskHandler) MoveTask(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	var placement models.TaskPlacement
	if err := database.GetDB().Where("task_id = ? AND board_id = ?", taskID, boardID).First(&placement).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task is not placed on this board"})
	}

	var req models.MoveTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	columnID, err := models.StrToID(req.ColumnID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	db := database.GetDB()
	oldColumnID := placement.ColumnID
	oldPosition := placement.Position

	if oldColumnID == columnID {
		if req.Position > oldPosition {
			db.Model(&models.TaskPlacement{}).
				Where("column_id = ? AND position > ? AND position <= ?", columnID, oldPosition, req.Position).
				Update("position", gorm.Expr("position - 1"))
		} else if req.Position < oldPosition {
			db.Model(&models.TaskPlacement{}).
				Where("column_id = ? AND position >= ? AND position < ?", columnID, req.Position, oldPosition).
				Update("position", gorm.Expr("position + 1"))
		}
	} else {
		db.Model(&models.TaskPlacement{}).
			Where("column_id = ? AND position > ?", oldColumnID, oldPosition).
			Update("position", gorm.Expr("position - 1"))
		db.Model(&models.TaskPlacement{}).
			Where("column_id = ? AND position >= ?", columnID, req.Position).
			Update("position", gorm.Expr("position + 1"))
	}

	placement.ColumnID = columnID
	placement.Position = req.Position
	if err := db.Save(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error moving task"})
	}

	task.UpdatedBy = currentUserID(c)
	db.Save(&task)

	return c.JSON(loadSingleTaskDTO(task, &placement))
}
```

This is a literal translation of the current gap-open/gap-close `gorm.Expr` logic, just keyed on `task_placements.column_id`/`position` instead of `tasks.column_id`/`position`.

- [ ] **Step 21: Add `PlaceTask`**

```go
func (h *TaskHandler) PlaceTask(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	var req models.PlaceTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	columnID, err := models.StrToID(req.ColumnID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	var existing models.TaskPlacement
	if err := database.GetDB().Where("task_id = ? AND board_id = ?", taskID, boardID).First(&existing).Error; err == nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Task is already placed on this board"})
	}

	var maxPosition struct{ Max int }
	database.GetDB().Model(&models.TaskPlacement{}).
		Select("COALESCE(MAX(position), -1) as max").
		Where("column_id = ?", columnID).
		Scan(&maxPosition)

	placement := models.TaskPlacement{
		TaskID:   uint(taskID),
		BoardID:  uint(boardID),
		ColumnID: columnID,
		Position: maxPosition.Max + 1,
	}
	if err := database.GetDB().Create(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error placing task"})
	}

	return c.Status(201).JSON(loadSingleTaskDTO(task, &placement))
}
```

- [ ] **Step 22: Add `RemovePlacement`**

```go
// RemovePlacement - DELETE /api/v1/projects/:id/boards/:boardId/tasks/:taskId
// Removes the task's placement from this board only. If this was the
// task's last placement, the task itself is deleted (comments/time
// entries cascade via existing FKs); otherwise only the placement is
// removed and the task survives on its other boards.
func (h *TaskHandler) RemovePlacement(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	db := database.GetDB()

	var placement models.TaskPlacement
	if err := db.Where("task_id = ? AND board_id = ?", taskID, boardID).First(&placement).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task is not placed on this board"})
	}

	if err := db.Delete(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error removing task from board"})
	}

	var remaining int64
	db.Model(&models.TaskPlacement{}).Where("task_id = ?", taskID).Count(&remaining)
	if remaining == 0 {
		if err := db.Delete(&models.Task{}, taskID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting task"})
		}
	}

	return c.JSON(fiber.Map{"success": true, "message": "Task removed from board successfully"})
}
```

- [ ] **Step 23: Rewrite `kanban_routes.go`**

```go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupKanbanRoutes(api fiber.Router) {
	kanbanHandler := handlers.NewKanbanHandler()
	boardHandler := handlers.NewBoardHandler()
	taskHandler := handlers.NewTaskHandler()
	commentHandler := handlers.NewTaskCommentHandler()
	timeEntryHandler := handlers.NewTaskTimeEntryHandler()

	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// Board entity CRUD
	projects.Get("/:id/boards", boardHandler.ListBoards)
	projects.Post("/:id/boards", middleware.RequirePermission("kanban.manage_columns"), boardHandler.CreateBoard)
	projects.Put("/:id/boards/:boardId", middleware.RequirePermission("kanban.manage_columns"), boardHandler.UpdateBoard)
	projects.Delete("/:id/boards/:boardId", middleware.RequirePermission("kanban.manage_columns"), boardHandler.DeleteBoard)

	// Board content (columns)
	projects.Get("/:id/boards/:boardId/kanban", kanbanHandler.GetBoard)
	projects.Put("/:id/boards/:boardId/kanban", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.UpdateBoard)
	projects.Post("/:id/boards/:boardId/kanban/columns", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.CreateColumn)
	projects.Put("/:id/boards/:boardId/kanban/columns/reorder", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.ReorderColumns)
	projects.Put("/:id/boards/:boardId/kanban/columns/:columnId", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.UpdateColumn)
	projects.Delete("/:id/boards/:boardId/kanban/columns/:columnId", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.DeleteColumn)

	// Placement management (board-scoped)
	projects.Post("/:id/boards/:boardId/tasks/:taskId/place", middleware.RequirePermission("tasks.move"), taskHandler.PlaceTask)
	projects.Put("/:id/boards/:boardId/tasks/:taskId/move", middleware.RequirePermission("tasks.move"), taskHandler.MoveTask)
	projects.Delete("/:id/boards/:boardId/tasks/:taskId", middleware.RequirePermission("tasks.delete"), taskHandler.RemovePlacement)

	// Project-scoped tasks (board-independent)
	projects.Get("/:id/tasks", taskHandler.GetTasks)
	projects.Get("/:id/tasks/:taskId", taskHandler.GetTask)
	projects.Post("/:id/tasks", middleware.RequirePermission("tasks.create"), taskHandler.CreateTask)
	projects.Put("/:id/tasks/:taskId", middleware.RequirePermission("tasks.update"), taskHandler.UpdateTask)
	projects.Delete("/:id/tasks/:taskId", middleware.RequirePermission("tasks.delete"), taskHandler.DeleteTask)

	// Comments
	projects.Get("/:id/tasks/:taskId/comments", commentHandler.GetComments)
	projects.Post("/:id/tasks/:taskId/comments", commentHandler.CreateComment)
	projects.Put("/:id/comments/:commentId", commentHandler.UpdateComment)
	projects.Delete("/:id/comments/:commentId", commentHandler.DeleteComment)

	// Time entries
	projects.Get("/:id/time-entries", middleware.RequirePermission("time_tracking.view"), timeEntryHandler.GetProjectTimeEntries)
	projects.Get("/:id/tasks/:taskId/time-entries", middleware.RequirePermission("time_tracking.view"), timeEntryHandler.GetTaskTimeEntries)
	projects.Post("/:id/tasks/:taskId/time-entries", middleware.RequirePermission("time_tracking.edit"), timeEntryHandler.CreateTimeEntry)
	projects.Put("/:id/time-entries/:entryId", middleware.RequirePermission("time_tracking.edit"), timeEntryHandler.UpdateTimeEntry)
	projects.Delete("/:id/time-entries/:entryId", middleware.RequirePermission("time_tracking.edit"), timeEntryHandler.DeleteTimeEntry)
}
```

Keep the exact handler constructor names, permission strings, and comment/time-entry method names as they exist in the current file — only the board/task/placement route section above changes shape.

- [ ] **Step 24: Build**

```bash
docker exec devbridge_backend go build ./...
```

Expected: no errors. Fix any signature mismatches against the actual current field/method names in `kanban.go`/`kanban_dto_builder.go`/`task_handler.go` if they differ from what's shown above, keeping the same approach (placement-conditional DTOs, board-scoped queries).

- [ ] **Step 25: Restart the backend and smoke-test with curl**

```bash
docker restart devbridge_backend
sleep 3

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"john@example.com","password":"TestPass123!"}' | jq -r .token)

# List boards for project 2 (should show the migrated "Main Board")
curl -s http://localhost:8080/api/v1/projects/2/boards -H "Authorization: Bearer $TOKEN" | jq .

BOARD_ID=$(curl -s http://localhost:8080/api/v1/projects/2/boards -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')

# Board content
curl -s http://localhost:8080/api/v1/projects/2/boards/$BOARD_ID/kanban -H "Authorization: Bearer $TOKEN" | jq .

# Create a second board
curl -s -X POST http://localhost:8080/api/v1/projects/2/boards -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"name":"Sprint 12","position":1}' | jq .
```

Expected: 200/201 responses, `Main Board` present, new board seeded with 4 default columns when fetched via its own `.../kanban` endpoint.

- [ ] **Step 26: Commit**

```bash
git add backend/internal/models/kanban.go backend/internal/handlers/kanban_dto_builder.go backend/internal/handlers/kanban_handler.go backend/internal/handlers/board_handler.go backend/internal/handlers/task_handler.go backend/internal/routes/kanban_routes.go
git commit -m "feat: multi-board data model, board CRUD, and placement-scoped task handlers"
```

---

## Frontend

### Task 3: Types — `Board`, board-scoped fields

**Files:**
- Modify: `frontend/src/types/kanban/kanban.types.ts`
- Modify: `frontend/src/types/kanban/task.types.ts`

**Interfaces:**
- Consumes: `BoardDTO`/`KanbanBoardDTO` shapes from Task 2.
- Produces: `Board` type and `KanbanBoard.name`, `CreateTaskData.boardId` — consumed by Tasks 4-8.

- [ ] **Step 1: Add `Board` type and `name` to `KanbanBoard`**

In `frontend/src/types/kanban/kanban.types.ts`, add `Name` to `KanbanBoard` and a new `Board` interface:

```typescript
export interface KanbanBoard {
    id: string;
    projectId: string;
    name: string;
    columns: KanbanColumn[];
    settings: KanbanSettings;
    createdAt: string;
    updatedAt: string;
}

export interface Board {
    id: string;
    projectId: string;
    name: string;
    position: number;
    createdAt: string;
    updatedAt: string;
}
```

(Replace the existing `KanbanBoard` interface in place; add `Board` right after it.)

- [ ] **Step 2: Add `ADD_TO_BOARD` to `ModalType`**

```typescript
export enum ModalType {
    TASK_EDIT = 'task_edit',
    COMMENTS = 'comments',
    TIME_LOG = 'time_log',
    COLUMN_SETTINGS = 'column_settings',
    ADD_TO_BOARD = 'add_to_board',
}
```

- [ ] **Step 3: Add `boardId` to `CreateTaskData`**

In `frontend/src/types/kanban/task.types.ts`:

```typescript
export interface CreateTaskData {
    title: string;
    description: string;
    htmlDescription?: string;
    priority: TaskPriority;
    columnId: string;
    boardId: string;
    assigneeId?: string;
    estimatedHours?: number;
    tags?: string[];
    dueDate?: string;
}
```

- [ ] **Step 4: Typecheck**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: new errors only in files not yet updated (`kanban-provider.tsx`, hooks, etc. — expected until Tasks 4-8 land). Confirm no errors originate from the two files touched in this task.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/kanban/kanban.types.ts frontend/src/types/kanban/task.types.ts
git commit -m "feat: add Board type and boardId to task types"
```

---

### Task 4: Services — board-scoped kanban/task calls, new `board.service.ts`

**Files:**
- Modify: `frontend/src/services/kanban/kanban.service.ts`
- Modify: `frontend/src/services/kanban/task.service.ts`
- Create: `frontend/src/services/kanban/board.service.ts`
- Modify: `frontend/src/services/kanban/index.ts`

**Interfaces:**
- Consumes: `Board`/`KanbanBoard`/`CreateTaskData` types from Task 3; route paths from Task 2 Step 23.
- Produces: `boardService.{listBoards,createBoard,updateBoard,deleteBoard}`, `kanbanService.*` all taking `boardId` after `projectId`, `taskService.{createTask,moveTask,placeTask,removePlacement}` — consumed by Task 6 hooks/provider and Task 8 components.

- [ ] **Step 1: Rewrite `kanban.service.ts` with `boardId`**

```typescript
// services/kanban/kanban.service.ts
import { apiClient } from '@/lib/api';
import type { KanbanBoard } from '@/types/kanban';

/**
 * Kanban board content service
 * Single Responsibility: board-scoped column/settings API operations
 */
export const kanbanService = {
    async getBoard(projectId: string, boardId: string): Promise<KanbanBoard> {
        return apiClient.get(`/projects/${projectId}/boards/${boardId}/kanban`);
    },

    async updateBoard(
        projectId: string,
        boardId: string,
        updates: Partial<KanbanBoard>
    ): Promise<KanbanBoard> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/kanban`, updates);
    },

    async createColumn(projectId: string, boardId: string, data: {
        title: string;
        color: string;
        position: number;
        maxTasks?: number;
    }) {
        return apiClient.post(`/projects/${projectId}/boards/${boardId}/kanban/columns`, data);
    },

    async updateColumn(
        projectId: string,
        boardId: string,
        columnId: string,
        data: {
            title?: string;
            color?: string;
            position?: number;
            maxTasks?: number;
        }
    ) {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/kanban/columns/${columnId}`, data);
    },

    async deleteColumn(projectId: string, boardId: string, columnId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/boards/${boardId}/kanban/columns/${columnId}`);
    },

    async reorderColumns(projectId: string, boardId: string, columnOrders: {
        columnId: string;
        position: number;
    }[]): Promise<void> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/kanban/columns/reorder`, {
            orders: columnOrders
        });
    }
};
```

- [ ] **Step 2: Rewrite `task.service.ts`'s `moveTask`, add `placeTask`/`removePlacement`**

Replace the `moveTask` method and add two new ones (keep `getTasks`, `getTask`, `createTask`, `updateTask`, `deleteTask`, `bulkUpdateTasks`, `duplicateTask`, `archiveTask`, `restoreTask` exactly as they are):

```typescript
    /**
     * Move task within/across columns of a specific board
     */
    async moveTask(
        projectId: string,
        boardId: string,
        taskId: string,
        data: MoveTaskData
    ): Promise<Task> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}/tasks/${taskId}/move`, data);
    },

    /**
     * Add an existing task to a board/column as a new placement
     */
    async placeTask(
        projectId: string,
        boardId: string,
        taskId: string,
        data: { columnId: string }
    ): Promise<Task> {
        return apiClient.post(`/projects/${projectId}/boards/${boardId}/tasks/${taskId}/place`, data);
    },

    /**
     * Remove the task's placement from a board only (task survives elsewhere)
     */
    async removePlacement(projectId: string, boardId: string, taskId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/boards/${boardId}/tasks/${taskId}`);
    },
```

(`createTask` already spreads `...data` into the POST body, so once `CreateTaskData.boardId` exists from Task 3, `createTask(projectId, { ...formData, boardId })` needs no service-layer change.)

- [ ] **Step 3: Create `board.service.ts`**

```typescript
// services/kanban/board.service.ts
import { apiClient } from '@/lib/api';
import type { Board } from '@/types/kanban';

/**
 * Board entity service
 * Single Responsibility: board CRUD API operations
 */
export const boardService = {
    async listBoards(projectId: string): Promise<Board[]> {
        return apiClient.get(`/projects/${projectId}/boards`);
    },

    async createBoard(projectId: string, data: { name: string; position: number }): Promise<Board> {
        return apiClient.post(`/projects/${projectId}/boards`, data);
    },

    async updateBoard(projectId: string, boardId: string, data: { name?: string; position?: number }): Promise<Board> {
        return apiClient.put(`/projects/${projectId}/boards/${boardId}`, data);
    },

    async deleteBoard(projectId: string, boardId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/boards/${boardId}`);
    }
};
```

- [ ] **Step 4: Export `boardService` from the services index**

In `frontend/src/services/kanban/index.ts`, add:

```typescript
export { boardService } from './board.service';
```

(alongside the existing `kanbanService`, `taskService`, `commentService`, `timeEntryService` exports).

- [ ] **Step 5: Typecheck**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: errors remain in call sites not yet updated (`kanban-provider.tsx`, `use-kanban.ts`, `use-tasks.ts`, `column-settings-modal.tsx`, `kanban-board.tsx`) — confirm no errors in the three service files or the index.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/services/kanban/kanban.service.ts frontend/src/services/kanban/task.service.ts frontend/src/services/kanban/board.service.ts frontend/src/services/kanban/index.ts
git commit -m "feat: board-scope kanban/task services, add board.service"
```

---

### Task 5: Store — additive `boards` slice

**Files:**
- Modify: `frontend/src/stores/kanban/kanban.store.ts`

**Interfaces:**
- Consumes: `Board` type from Task 3.
- Produces: `useKanbanStore().{boards, setBoards, addBoardToStore, updateBoardInStore, removeBoardFromStore}` — consumed by the boards-list page in Task 7.

This is purely additive — no existing state field, action name, or behavior changes. `board` (singular, current board's content) and `boards` (plural, the project's board list) are separate, coexisting pieces of state.

- [ ] **Step 1: Add `boards` to `KanbanState` and its actions to `KanbanActions`**

```typescript
import type {
    Task,
    TaskComment,
    TimeEntry,
    KanbanBoard,
    KanbanColumn,
    Board
} from '@/types/kanban';

interface KanbanState {
    // Board state
    board: KanbanBoard | null;
    columns: KanbanColumn[];

    // Project's board list (for the boards-list page)
    boards: Board[];

    // Tasks state
    tasks: Task[];
    selectedTaskId: string | null;

    // Comments state
    comments: TaskComment[];

    // Time entries state
    timeEntries: TimeEntry[];

    // UI state
    isLoading: boolean;
    error: string | null;

    // Optimistic updates
    optimisticUpdates: Map<string, any>;
}
```

```typescript
interface KanbanActions {
    // Board actions
    setBoard: (board: KanbanBoard) => void;
    updateBoard: (updates: Partial<KanbanBoard>) => void;
    clearBoard: () => void;

    // Board list actions
    setBoards: (boards: Board[]) => void;
    addBoardToStore: (board: Board) => void;
    updateBoardInStore: (boardId: string, updates: Partial<Board>) => void;
    removeBoardFromStore: (boardId: string) => void;

    // ... (rest of KanbanActions unchanged)
}
```

- [ ] **Step 2: Add `boards: []` to `initialState`**

```typescript
const initialState: KanbanState = {
    board: null,
    columns: [],
    boards: [],
    tasks: [],
    selectedTaskId: null,
    comments: [],
    timeEntries: [],
    isLoading: false,
    error: null,
    optimisticUpdates: new Map(),
};
```

- [ ] **Step 3: Add the four new actions inside the `immer((set, get) => ({ ... }))` body**

Insert right after the existing `clearBoard` action:

```typescript
                // Board list actions
                setBoards: (boards) => set((state) => {
                    state.boards = boards;
                }),

                addBoardToStore: (board) => set((state) => {
                    state.boards.push(board);
                }),

                updateBoardInStore: (boardId, updates) => set((state) => {
                    const index = state.boards.findIndex((b) => b.id === boardId);
                    if (index !== -1) {
                        Object.assign(state.boards[index], updates);
                    }
                }),

                removeBoardFromStore: (boardId) => set((state) => {
                    state.boards = state.boards.filter((b) => b.id !== boardId);
                }),

```

- [ ] **Step 4: Typecheck**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: no new errors from `kanban.store.ts` itself.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/kanban/kanban.store.ts
git commit -m "feat: add boards list slice to kanban store"
```

---

### Task 6: Hooks and provider — `boardId` threading, flat-tasks derivation

**Files:**
- Modify: `frontend/src/providers/kanban-provider.tsx`
- Modify: `frontend/src/hooks/kanban/use-kanban.ts`
- Modify: `frontend/src/hooks/kanban/use-tasks.ts`

**Interfaces:**
- Consumes: `kanbanService`/`taskService`/`boardService` from Task 4, store actions from Task 5.
- Produces: `KanbanProvider({ projectId, boardId, children })`, `useKanban(projectId, boardId)`, `useTasks(projectId, boardId): { ..., removeFromBoard, placeOnBoard }` — consumed by Task 7 pages and Task 8 components.

- [ ] **Step 1: Rewrite `KanbanProvider` to be board-scoped and derive flat tasks from board columns**

```typescript
'use client';

import React, { createContext, useContext, useEffect } from 'react';
import { useKanbanStore } from '@/stores/kanban/kanban.store';
import { kanbanService } from '@/services/kanban';
import type { KanbanBoard } from '@/types/kanban';

interface KanbanContextValue {
    projectId: string;
    boardId: string;
    isLoading: boolean;
    error: string | null;
}

const KanbanContext = createContext<KanbanContextValue | null>(null);

interface KanbanProviderProps {
    projectId: string;
    boardId: string;
    children: React.ReactNode;
}

export const KanbanProvider: React.FC<KanbanProviderProps> = ({
                                                                  projectId,
                                                                  boardId,
                                                                  children
                                                              }) => {
    const {
        setBoard,
        setTasks,
        setComments,
        setTimeEntries,
        setLoading,
        setError,
        isLoading,
        error
    } = useKanbanStore();

    useEffect(() => {
        const loadKanbanData = async () => {
            if (!projectId || !boardId) return;

            setLoading(true);
            setError(null);

            try {
                const board = await kanbanService.getBoard(projectId, boardId);
                const tasks = board.columns.flatMap((col) => col.tasks);

                setBoard(board);
                setTasks(tasks);
                setComments(tasks.flatMap((task) => task.comments ?? []));
                setTimeEntries(tasks.flatMap((task) => task.timeEntries ?? []));
            } catch (err) {
                console.error('Error loading kanban data:', err);
                setError(err instanceof Error ? err.message : 'Failed to load board data');
            } finally {
                setLoading(false);
            }
        };

        loadKanbanData();
    }, [projectId, boardId, setBoard, setTasks, setComments, setTimeEntries, setLoading, setError]);

    const contextValue: KanbanContextValue = {
        projectId,
        boardId,
        isLoading,
        error
    };

    return (
        <KanbanContext.Provider value={contextValue}>
            {children}
        </KanbanContext.Provider>
    );
};

export const useKanbanContext = (): KanbanContextValue => {
    const context = useContext(KanbanContext);
    if (!context) {
        throw new Error('useKanbanContext must be used within KanbanProvider');
    }
    return context;
};
```

This drops the `taskService.getTasks` call and the `createDefaultBoard` demo-data fallback entirely: the flat tasks array is now derived from `board.columns.flatMap(...)`, and a failed fetch surfaces as a real error (via `setError`) instead of silently rendering fake data — appropriate now that the board is a real, board-scoped entity rather than an implicit per-project singleton.

- [ ] **Step 2: Update `useKanban` to accept and use `boardId`**

```typescript
'use client';

import { useMemo } from 'react';
import { usePermissions } from '@/hooks/auth/use-permissions';
import { useKanbanStore } from '@/stores/kanban';
import { kanbanService } from '@/services/kanban';
import type {
    KanbanBoard,
    KanbanColumn,
    KanbanPermissions,
} from '@/types/kanban';

interface UseKanbanReturn {
    board: KanbanBoard | null;
    columns: KanbanColumn[];
    isLoading: boolean;
    error: string | null;
    permissions: KanbanPermissions;
    updateBoard: (updates: Partial<KanbanBoard>) => Promise<void>;
}

export const useKanban = (projectId: string, boardId: string): UseKanbanReturn => {
    const { hasPermission } = usePermissions();
    const { board, columns, isLoading, error, updateBoard: updateBoardInStore } = useKanbanStore();

    const permissions = useMemo((): KanbanPermissions => ({
        canCreateTasks: hasPermission('tasks:create', projectId),
        canEditTasks: hasPermission('tasks:edit', projectId),
        canDeleteTasks: hasPermission('tasks:delete', projectId),
        canMoveTasks: hasPermission('tasks:move', projectId),
        canManageColumns: hasPermission('kanban:manage_columns', projectId),
        canViewTimeTracking: hasPermission('time_tracking:view', projectId),
        canEditTimeTracking: hasPermission('time_tracking:edit', projectId),
    }), [hasPermission, projectId]);

    const updateBoard = async (updates: Partial<KanbanBoard>): Promise<void> => {
        if (!board || !permissions.canManageColumns) return;

        const updatedBoard = await kanbanService.updateBoard(projectId, boardId, updates);
        updateBoardInStore(updatedBoard);
    };

    return {
        board,
        columns,
        isLoading,
        error,
        permissions,
        updateBoard,
    };
};
```

- [ ] **Step 3: Update `use-tasks.ts` — add `boardId`, `removeFromBoard`, `placeOnBoard`**

Change the signature and `createTask`/`moveTask` bodies, and add two new functions (keep `filteredTasks`, `getTasksByColumn`, `getTask`, `updateTask`, `deleteTask` exactly as they are):

```typescript
export const useTasks = (projectId: string, boardId?: string): UseTasksReturn => {
    // ... existing state/store destructuring unchanged ...

    const createTask = useCallback(async (data: CreateTaskData): Promise<Task> => {
        setIsLoading(true);
        setError(null);

        try {
            const task = await taskService.createTask(projectId, { ...data, boardId: boardId ?? data.boardId });
            addTask(task);
            return task;
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Failed to create task';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, boardId, addTask]);

    // ... updateTask, deleteTask unchanged ...

    const moveTask = useCallback(async (
        taskId: string,
        data: MoveTaskData
    ): Promise<Task> => {
        if (!boardId) throw new Error('moveTask requires a boardId');

        moveTaskInStore(taskId, data.columnId, data.position);

        try {
            const task = await taskService.moveTask(projectId, boardId, taskId, data);
            updateTaskInStore(task);
            return task;
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Failed to move task';
            setError(errorMessage);
            throw err;
        }
    }, [projectId, boardId, moveTaskInStore, updateTaskInStore]);

    const removeFromBoard = useCallback(async (taskId: string): Promise<void> => {
        if (!boardId) throw new Error('removeFromBoard requires a boardId');

        setIsLoading(true);
        setError(null);

        try {
            await taskService.removePlacement(projectId, boardId, taskId);
            deleteTaskFromStore(taskId);
        } catch (err) {
            const errorMessage = err instanceof Error ? err.message : 'Failed to remove task from board';
            setError(errorMessage);
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, boardId, deleteTaskFromStore]);

    const placeOnBoard = useCallback(async (
        taskId: string,
        targetBoardId: string,
        columnId: string
    ): Promise<Task> => {
        return taskService.placeTask(projectId, targetBoardId, taskId, { columnId });
    }, [projectId]);

    return {
        tasks: tasks.filter(task => task.projectId === projectId),
        filteredTasks,
        isLoading,
        error,
        createTask,
        updateTask,
        deleteTask,
        moveTask,
        removeFromBoard,
        placeOnBoard,
        setFilters,
        setSortOptions,
        getTasksByColumn,
        getTask,
    };
};
```

Add `removeFromBoard: (taskId: string) => Promise<void>` and `placeOnBoard: (taskId: string, targetBoardId: string, columnId: string) => Promise<Task>` to the `UseTasksReturn` interface at the top of the file. `boardId` is optional (`boardId?: string`) so `TaskForm`'s existing `useTasks('')` call (it only uses `getTask`, never `createTask`/`moveTask`) keeps compiling unchanged.

- [ ] **Step 4: Typecheck**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: remaining errors only in `kanban-board.tsx`/`column-settings-modal.tsx` call sites (fixed in Task 8) and the two page files (fixed in Task 7). Confirm no errors in the three files touched here.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/providers/kanban-provider.tsx frontend/src/hooks/kanban/use-kanban.ts frontend/src/hooks/kanban/use-tasks.ts
git commit -m "feat: board-scope kanban provider and hooks, derive flat tasks from board columns"
```

---

### Task 7: Pages — nav rename, boards-list page, board-scoped kanban page

**Files:**
- Modify: `frontend/src/components/dashboard/DashboardNav.tsx`
- Modify: `frontend/src/app/dashboard/board/page.tsx`
- Modify: `frontend/src/app/dashboard/board/[projectId]/page.tsx` (repurposed from kanban view to boards-list)
- Create: `frontend/src/app/dashboard/board/[projectId]/[boardId]/page.tsx` (new kanban view)

**Interfaces:**
- Consumes: `boardService` from Task 4, `KanbanProvider` from Task 6.
- Produces: nothing consumed by later tasks — this is the navigable surface.

- [ ] **Step 1: Rename nav label**

In `frontend/src/components/dashboard/DashboardNav.tsx`, find the `links` array entry:

```typescript
{ href: '/dashboard/board', label: 'Board', icon: KanbanSquare, show: true },
```

and change `label: 'Board'` to `label: 'Projects'`.

- [ ] **Step 2: Update heading text on the projects-list page**

In `frontend/src/app/dashboard/board/page.tsx`, find:

```typescript
<h1 className="text-2xl font-bold text-gray-900">Board</h1>
<p className="text-gray-500 text-sm">Choose a project to open its board</p>
```

and change to:

```typescript
<h1 className="text-2xl font-bold text-gray-900">Projects</h1>
<p className="text-gray-500 text-sm">Choose a project to open its boards</p>
```

(Everything else in this file — the project table, `router.push` navigation — stays unchanged; it still routes to `/dashboard/board/${project.id}`, which now lands on the repurposed boards-list page from Step 3.)

- [ ] **Step 3: Repurpose `[projectId]/page.tsx` into a boards-list page**

Replace the entire file content:

```typescript
'use client';

import React from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { boardService } from '@/services/kanban';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, ArrowLeft, KanbanSquare } from 'lucide-react';
import type { Board } from '@/types/kanban';

export default function BoardsListPage() {
    const { projectId } = useParams<{ projectId: string }>();
    const router = useRouter();
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [isLoading, setIsLoading] = React.useState(true);
    const [newBoardName, setNewBoardName] = React.useState('');
    const [isCreating, setIsCreating] = React.useState(false);

    React.useEffect(() => {
        boardService.listBoards(projectId)
            .then((data) => setBoards(data))
            .finally(() => setIsLoading(false));
    }, [projectId]);

    const handleCreate = async () => {
        if (!newBoardName.trim()) return;
        setIsCreating(true);
        try {
            const board = await boardService.createBoard(projectId, {
                name: newBoardName.trim(),
                position: boards.length,
            });
            setBoards((prev) => [...prev, board]);
            setNewBoardName('');
        } finally {
            setIsCreating(false);
        }
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    return (
        <div className="p-6">
            <Link href="/dashboard/board" className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 mb-4">
                <ArrowLeft size={14} /> Back to projects
            </Link>

            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold text-gray-900">Boards</h1>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                {boards.map((board) => (
                    <button
                        key={board.id}
                        onClick={() => router.push(`/dashboard/board/${projectId}/${board.id}`)}
                        className="flex items-center gap-3 p-4 bg-white border border-gray-200 rounded-lg shadow-sm hover:shadow-md hover:border-blue-300 transition-all text-left"
                    >
                        <KanbanSquare className="text-blue-600" size={20} />
                        <span className="font-medium text-gray-900">{board.name}</span>
                    </button>
                ))}
            </div>

            <div className="flex items-center gap-2 max-w-sm">
                <Input
                    value={newBoardName}
                    onChange={setNewBoardName}
                    placeholder="New board name..."
                    className="flex-1"
                />
                <Button icon={Plus} onClick={handleCreate} disabled={!newBoardName.trim()} loading={isCreating}>
                    New board
                </Button>
            </div>
        </div>
    );
}
```

- [ ] **Step 4: Create the new board-scoped kanban page**

```typescript
// frontend/src/app/dashboard/board/[projectId]/[boardId]/page.tsx
'use client';

import React from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { KanbanProvider } from '@/providers/kanban-provider';
import { KanbanBoard } from '@/components/kanban';

export default function BoardPage() {
    const { projectId, boardId } = useParams<{ projectId: string; boardId: string }>();

    return (
        <div className="h-full flex flex-col">
            <div className="px-6 pt-4">
                <Link
                    href={`/dashboard/board/${projectId}`}
                    className="text-sm text-gray-500 hover:text-gray-700"
                >
                    ← Back to boards
                </Link>
            </div>
            <KanbanProvider projectId={projectId} boardId={boardId}>
                <KanbanBoard />
            </KanbanProvider>
        </div>
    );
}
```

- [ ] **Step 5: Typecheck**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: remaining errors only in `kanban-board.tsx`, `column-settings-modal.tsx`, `task-card.tsx`, `kanban-column.tsx`, `task-form.tsx` (fixed in Task 8).

- [ ] **Step 6: Manual verification**

```bash
docker logs devbridge_frontend --tail 20
```

Start the dev server if not running (`docker exec devbridge_frontend npm run dev`, or confirm it's already running), then in a browser: log in, click "Projects" in the nav (confirm the renamed label), open a project, confirm the new boards-list page renders "Main Board", click it, confirm it 404s or errors gracefully (expected — `KanbanBoard`'s internals aren't board-scoped yet until Task 8).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/dashboard/DashboardNav.tsx frontend/src/app/dashboard/board/page.tsx "frontend/src/app/dashboard/board/[projectId]/page.tsx" "frontend/src/app/dashboard/board/[projectId]/[boardId]/page.tsx"
git commit -m "feat: repurpose project board route into boards-list, add board-scoped kanban page"
```

---

### Task 8: Components — board-aware kanban board, add/remove-from-board actions, delete permanently

**Files:**
- Modify: `frontend/src/components/kanban/kanban-board.tsx`
- Modify: `frontend/src/components/kanban/kanban-column.tsx`
- Modify: `frontend/src/components/kanban/task-card.tsx`
- Modify: `frontend/src/components/kanban/task-form.tsx`
- Modify: `frontend/src/components/kanban/column-settings-modal.tsx`
- Create: `frontend/src/components/kanban/add-to-board-modal.tsx`
- Modify: `frontend/src/components/kanban/index.ts`

**Interfaces:**
- Consumes: `useKanban(projectId, boardId)`, `useTasks(projectId, boardId)` from Task 6; `boardService`, `kanbanService`, `taskService` from Task 4; `Board` type from Task 3.
- Produces: nothing consumed by later tasks — this is the final UI wiring, verified in Task 9.

- [ ] **Step 1: Rewrite `kanban-board.tsx` to read `boardId` and wire the new actions**

```typescript
'use client';

import React from 'react';
import { useParams } from 'next/navigation';
import { KanbanColumn } from './kanban-column';
import { TaskForm } from './task-form';
import { CommentSection } from './comment-section';
import { TimeTracker } from './time-tracker';
import { ColumnSettingsModal } from './column-settings-modal';
import { AddToBoardModal } from './add-to-board-modal';
import { Modal } from '@/components/ui/modal';
import { Button } from '@/components/ui/button';
import { useKanban, useTasks, useDragDrop } from '@/hooks/kanban';
import { ModalType } from '@/types/kanban';
import { Plus, Settings } from 'lucide-react';

export const KanbanBoard: React.FC = () => {
    const { projectId, boardId } = useParams<{ projectId: string; boardId: string }>();

    const { board, columns, isLoading, error, permissions } = useKanban(projectId, boardId);
    const { getTasksByColumn, createTask, updateTask, deleteTask, moveTask, removeFromBoard } = useTasks(projectId, boardId);
    const dragDrop = useDragDrop();

    const [activeModal, setActiveModal] = React.useState<{
        type: ModalType | null;
        taskId?: string;
        columnId?: string;
    }>({ type: null });

    const handleAddTask = (columnId: string) => {
        if (!permissions.canCreateTasks) return;
        setActiveModal({ type: ModalType.TASK_EDIT, columnId });
    };

    const handleEditTask = (taskId: string) => {
        if (!permissions.canEditTasks) return;
        setActiveModal({ type: ModalType.TASK_EDIT, taskId });
    };

    const handleOpenComments = (taskId: string) => {
        setActiveModal({ type: ModalType.COMMENTS, taskId });
    };

    const handleOpenTimeLog = (taskId: string) => {
        if (!permissions.canViewTimeTracking) return;
        setActiveModal({ type: ModalType.TIME_LOG, taskId });
    };

    const handleOpenSettings = () => {
        if (!permissions.canManageColumns) return;
        setActiveModal({ type: ModalType.COLUMN_SETTINGS });
    };

    const handleAddToBoard = (taskId: string) => {
        setActiveModal({ type: ModalType.ADD_TO_BOARD, taskId });
    };

    const handleTaskMove = async (result: any) => {
        if (!permissions.canMoveTasks) return;

        try {
            await moveTask(result.taskId, {
                columnId: result.toColumnId,
                position: result.newPosition || 0
            });
        } catch (error) {
            console.error('Failed to move task:', error);
        }
    };

    const closeModal = () => {
        setActiveModal({ type: null });
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-64">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
        );
    }

    if (error) {
        return (
            <div className="text-center text-red-600 p-4">
                <p>Error loading kanban board: {error}</p>
                <Button onClick={() => window.location.reload()} className="mt-2">
                    Retry
                </Button>
            </div>
        );
    }

    return (
        <div className="h-full flex flex-col">
            <div className="flex items-center justify-between p-6 border-b">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">{board?.name ?? 'Kanban Board'}</h1>
                    <p className="text-gray-600 mt-1">Manage your project tasks</p>
                </div>

                <div className="flex items-center gap-2">
                    {permissions.canManageColumns && (
                        <Button variant="ghost" icon={Settings} onClick={handleOpenSettings}>
                            Settings
                        </Button>
                    )}
                </div>
            </div>

            <div className="flex-1 overflow-x-auto">
                <div className="flex gap-6 p-6 min-w-max">
                    {columns.map((column) => (
                        <KanbanColumn
                            key={column.id}
                            column={column}
                            tasks={getTasksByColumn(column.id)}
                            permissions={permissions}
                            isHighlighted={dragDrop.isColumnHighlighted(column.id)}
                            dragHandlers={{
                                onDragOver: dragDrop.handleDragOver,
                                onDragEnter: (e) => dragDrop.handleDragEnter(e, column.id),
                                onDragLeave: dragDrop.handleDragLeave,
                                onDrop: (e) => {
                                    const result = dragDrop.handleDrop(e, column.id);
                                    if (result) handleTaskMove(result);
                                }
                            }}
                            onAddTask={() => handleAddTask(column.id)}
                            onEditTask={handleEditTask}
                            onRemoveTask={removeFromBoard}
                            onAddToBoard={handleAddToBoard}
                            onOpenComments={handleOpenComments}
                            onOpenTimeLog={handleOpenTimeLog}
                            onDragStart={(task) => dragDrop.startDrag(task, column.id)}
                            onDragEnd={dragDrop.endDrag}
                        />
                    ))}
                </div>
            </div>

            <Modal
                isOpen={activeModal.type === ModalType.TASK_EDIT}
                onClose={closeModal}
                title={activeModal.taskId ? 'Edit Task' : 'Create Task'}
                size="lg"
            >
                <TaskForm
                    taskId={activeModal.taskId}
                    columnId={activeModal.columnId}
                    onSubmit={async (data) => {
                        if (activeModal.taskId) {
                            await updateTask(activeModal.taskId, data);
                        } else if (activeModal.columnId) {
                            await createTask({ ...data, columnId: activeModal.columnId, boardId });
                        }
                        closeModal();
                    }}
                    onCancel={closeModal}
                    onDeletePermanently={activeModal.taskId ? async () => {
                        await deleteTask(activeModal.taskId!);
                        closeModal();
                    } : undefined}
                />
            </Modal>

            <Modal
                isOpen={activeModal.type === ModalType.COMMENTS}
                onClose={closeModal}
                title="Comments"
                size="lg"
            >
                {activeModal.taskId && (
                    <CommentSection taskId={activeModal.taskId} />
                )}
            </Modal>

            <Modal
                isOpen={activeModal.type === ModalType.TIME_LOG}
                onClose={closeModal}
                title="Time Tracking"
                size="lg"
            >
                {activeModal.taskId && (
                    <TimeTracker taskId={activeModal.taskId} />
                )}
            </Modal>

            <Modal
                isOpen={activeModal.type === ModalType.COLUMN_SETTINGS}
                onClose={closeModal}
                title="Column Settings"
                size="lg"
            >
                <ColumnSettingsModal projectId={projectId} boardId={boardId} />
            </Modal>

            <Modal
                isOpen={activeModal.type === ModalType.ADD_TO_BOARD}
                onClose={closeModal}
                title="Add to another board"
                size="md"
            >
                {activeModal.taskId && (
                    <AddToBoardModal
                        projectId={projectId}
                        currentBoardId={boardId}
                        taskId={activeModal.taskId}
                        onDone={closeModal}
                    />
                )}
            </Modal>
        </div>
    );
};
```

- [ ] **Step 2: Update `KanbanColumnProps` and task-card wiring in `kanban-column.tsx`**

Rename `onDeleteTask` to `onRemoveTask`, add `onAddToBoard`:

```typescript
interface KanbanColumnProps {
    column: Column;
    tasks: Task[];
    permissions: KanbanPermissions;
    isHighlighted: boolean;
    dragHandlers: {
        onDragOver: (e: React.DragEvent) => void;
        onDragEnter: (e: React.DragEvent) => void;
        onDragLeave: (e: React.DragEvent) => void;
        onDrop: (e: React.DragEvent) => void;
    };
    onAddTask: () => void;
    onEditTask: (taskId: string) => void;
    onRemoveTask: (taskId: string) => void;
    onAddToBoard: (taskId: string) => void;
    onOpenComments: (taskId: string) => void;
    onOpenTimeLog: (taskId: string) => void;
    onDragStart: (task: Task) => void;
    onDragEnd: () => void;
}
```

Update the destructured props (`onRemoveTask`, `onAddToBoard` replacing/joining `onDeleteTask`) and the `TaskCard` render:

```typescript
                    tasks.map((task) => (
                        <TaskCard
                            key={task.id}
                            task={task}
                            permissions={permissions}
                            onEdit={() => onEditTask(task.id)}
                            onRemove={() => onRemoveTask(task.id)}
                            onAddToBoard={() => onAddToBoard(task.id)}
                            onOpenComments={() => onOpenComments(task.id)}
                            onOpenTimeLog={() => onOpenTimeLog(task.id)}
                            onDragStart={() => onDragStart(task)}
                            onDragEnd={onDragEnd}
                        />
                    ))
```

- [ ] **Step 3: Update `task-card.tsx` — rename delete to remove, add "Add to another board" action**

Rename the prop and add a new one:

```typescript
interface TaskCardProps {
    task: Task;
    permissions: KanbanPermissions;
    onEdit: () => void;
    onRemove: () => void;
    onAddToBoard: () => void;
    onOpenComments: () => void;
    onOpenTimeLog: () => void;
    onDragStart: () => void;
    onDragEnd: () => void;
}
```

Add `FolderPlus` to the lucide-react import list:

```typescript
import {
    Edit2,
    MessageSquare,
    Timer,
    Trash2,
    FolderPlus,
    GripVertical,
    User,
    Clock,
    Calendar,
    AlertCircle
} from 'lucide-react';
```

Update the destructured props and the Actions footer (replace the existing delete button block):

```typescript
export const TaskCard: React.FC<TaskCardProps> = ({
                                                      task,
                                                      permissions,
                                                      onEdit,
                                                      onRemove,
                                                      onAddToBoard,
                                                      onOpenComments,
                                                      onOpenTimeLog,
                                                      onDragStart,
                                                      onDragEnd
                                                  }) => {
```

```typescript
                {permissions.canDeleteTasks && (
                    <div className="flex items-center gap-1">
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                onAddToBoard();
                            }}
                            icon={FolderPlus}
                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                        />
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) => {
                                e.stopPropagation();
                                if (confirm('Remove this task from this board?')) {
                                    onRemove();
                                }
                            }}
                            icon={Trash2}
                            className="opacity-0 group-hover:opacity-100 transition-opacity text-red-600 hover:bg-red-50"
                        />
                    </div>
                )}
```

(The rest of `task-card.tsx` — `handleDragStart`'s `dataTransfer` payload, priority colors/icons, description, tags, progress bar, meta info — is unchanged.)

- [ ] **Step 4: Add "Delete permanently" to `task-form.tsx`**

Add a new optional prop and render it as a danger action, only visible when editing an existing task:

```typescript
interface TaskFormProps {
    taskId?: string;
    columnId?: string;
    onSubmit: (data: any) => Promise<void>;
    onCancel: () => void;
    onDeletePermanently?: () => Promise<void>;
}

export const TaskForm: React.FC<TaskFormProps> = ({
                                                      taskId,
                                                      columnId,
                                                      onSubmit,
                                                      onCancel,
                                                      onDeletePermanently
                                                  }) => {
```

Add an `isDeleting` state and a handler, then update the Actions row:

```typescript
    const [isDeleting, setIsDeleting] = useState(false);

    const handleDeletePermanently = async () => {
        if (!onDeletePermanently) return;
        if (!confirm('Delete this task permanently? It will be removed from every board it appears on.')) return;

        setIsDeleting(true);
        try {
            await onDeletePermanently();
        } finally {
            setIsDeleting(false);
        }
    };
```

```typescript
            {/* Actions */}
            <div className="flex items-center justify-between gap-3 pt-4 border-t">
                {taskId && onDeletePermanently ? (
                    <Button
                        type="button"
                        variant="ghost"
                        onClick={handleDeletePermanently}
                        loading={isDeleting}
                        icon={Trash2}
                        className="text-red-600 hover:bg-red-50"
                    >
                        Delete permanently
                    </Button>
                ) : <div />}

                <div className="flex items-center gap-3">
                    <Button
                        type="button"
                        variant="ghost"
                        onClick={onCancel}
                        icon={X}
                    >
                        Cancel
                    </Button>
                    <Button
                        type="submit"
                        loading={isLoading}
                        icon={Save}
                    >
                        {taskId ? 'Update Task' : 'Create Task'}
                    </Button>
                </div>
            </div>
```

Add `Trash2` to the existing `lucide-react` import: `import { Save, X, Trash2 } from 'lucide-react';`.

- [ ] **Step 5: Thread `boardId` through `column-settings-modal.tsx`, update delete-column copy**

```typescript
interface ColumnSettingsModalProps {
    projectId: string;
    boardId: string;
}

export const ColumnSettingsModal: React.FC<ColumnSettingsModalProps> = ({ projectId, boardId }) => {
```

Update every `kanbanService.*` call site to pass `boardId`:

```typescript
    const persistColumn = async (columnId: string, updates: Partial<Pick<KanbanColumn, 'title' | 'color' | 'maxTasks'>>) => {
        setError(null);
        setSavingId(columnId);
        try {
            await kanbanService.updateColumn(projectId, boardId, columnId, updates);
            updateColumnInStore(columnId, updates);
        } catch {
            setError('Failed to update column.');
        } finally {
            setSavingId(null);
        }
    };
```

```typescript
        setError(null);
        try {
            await kanbanService.reorderColumns(projectId, boardId, orders);
            setColumns(reordered.map((col, i) => ({ ...col, position: i })));
        } catch {
            setError('Failed to reorder columns.');
        }
```

```typescript
        setError(null);
        setSavingId(column.id);
        try {
            await kanbanService.deleteColumn(projectId, boardId, column.id);
            deleteColumnFromStore(column.id);
        } catch {
            setError('Failed to delete column.');
        } finally {
            setSavingId(null);
        }
```

```typescript
        setError(null);
        try {
            const column = await kanbanService.createColumn(projectId, boardId, {
                title: newTitle.trim(),
                color: newColor,
                position: columns.length,
            }) as KanbanColumn;
            addColumnToStore(column);
            setNewTitle('');
            setNewColor(COLUMN_COLORS[0]);
        } catch {
            setError('Failed to create column.');
        }
```

Update the delete confirmation copy to reflect placement semantics:

```typescript
    const handleDelete = async (column: KanbanColumn) => {
        const taskCount = column.tasks?.length ?? 0;
        const confirmMessage = taskCount > 0
            ? `Delete "${column.title}"? ${taskCount} task(s) in it will be permanently deleted if this is their only remaining placement; tasks placed on other boards will survive there.`
            : `Delete "${column.title}"?`;
        if (!window.confirm(confirmMessage)) return;

        // ... unchanged body
    };
```

- [ ] **Step 6: Create `add-to-board-modal.tsx`**

```typescript
'use client';

import React from 'react';
import { Button } from '@/components/ui/button';
import { Select } from '@/components/ui/select';
import { boardService, kanbanService, taskService } from '@/services/kanban';
import type { Board, KanbanColumn as ColumnType } from '@/types/kanban';

interface AddToBoardModalProps {
    projectId: string;
    currentBoardId: string;
    taskId: string;
    onDone: () => void;
}

export const AddToBoardModal: React.FC<AddToBoardModalProps> = ({ projectId, currentBoardId, taskId, onDone }) => {
    const [boards, setBoards] = React.useState<Board[]>([]);
    const [selectedBoardId, setSelectedBoardId] = React.useState('');
    const [columns, setColumns] = React.useState<ColumnType[]>([]);
    const [selectedColumnId, setSelectedColumnId] = React.useState('');
    const [isLoading, setIsLoading] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);

    React.useEffect(() => {
        boardService.listBoards(projectId).then((all) => {
            const targets = all.filter((b) => b.id !== currentBoardId);
            setBoards(targets);
            if (targets.length > 0) setSelectedBoardId(targets[0].id);
        });
    }, [projectId, currentBoardId]);

    React.useEffect(() => {
        if (!selectedBoardId) {
            setColumns([]);
            setSelectedColumnId('');
            return;
        }
        kanbanService.getBoard(projectId, selectedBoardId).then((board) => {
            setColumns(board.columns);
            setSelectedColumnId(board.columns[0]?.id ?? '');
        });
    }, [projectId, selectedBoardId]);

    const handleSubmit = async () => {
        if (!selectedBoardId || !selectedColumnId) return;
        setIsLoading(true);
        setError(null);
        try {
            await taskService.placeTask(projectId, selectedBoardId, taskId, { columnId: selectedColumnId });
            onDone();
        } catch {
            setError('Failed to add task to board.');
        } finally {
            setIsLoading(false);
        }
    };

    if (boards.length === 0) {
        return <p className="text-sm text-gray-500">No other boards in this project yet.</p>;
    }

    return (
        <div className="space-y-4">
            {error && (
                <div className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            <Select
                label="Board"
                value={selectedBoardId}
                onChange={setSelectedBoardId}
                options={boards.map((b) => ({ value: b.id, label: b.name }))}
            />

            <Select
                label="Column"
                value={selectedColumnId}
                onChange={setSelectedColumnId}
                options={columns.map((c) => ({ value: c.id, label: c.title }))}
            />

            <div className="flex items-center justify-end gap-3 pt-4 border-t">
                <Button type="button" variant="ghost" onClick={onDone}>Cancel</Button>
                <Button type="button" onClick={handleSubmit} loading={isLoading} disabled={!selectedColumnId}>
                    Add to board
                </Button>
            </div>
        </div>
    );
};
```

- [ ] **Step 7: Export `AddToBoardModal` (optional, for symmetry — `ColumnSettingsModal` is imported directly today, keep the same pattern)**

No change needed to `frontend/src/components/kanban/index.ts` — `AddToBoardModal` is imported directly by relative path from `kanban-board.tsx`, matching how `ColumnSettingsModal` is already handled (it is *not* in the index either).

- [ ] **Step 8: Typecheck**

```bash
docker exec devbridge_frontend npx tsc --noEmit
```

Expected: zero errors across the whole frontend.

- [ ] **Step 9: Manual verification**

With both containers running (`docker exec devbridge_backend ...` server up, `docker exec devbridge_frontend npm run dev`), in a browser:
1. Log in, navigate to a project's boards-list page, confirm "Main Board" is listed.
2. Create a second board ("Sprint 12"), confirm it appears seeded with the 4 default columns.
3. On "Main Board", create a task, confirm it appears in its column.
4. Click the new "Add to another board" (folder-plus) icon on that task, select "Sprint 12" and a column, submit, confirm no console/network errors.
5. Navigate to "Sprint 12", confirm the same task appears there, move it to a different column (confirm the move persists on reload).
6. Navigate back to "Main Board", confirm the task's column there is unchanged by the move on "Sprint 12".
7. Click "Remove from this board" (trash icon) on "Main Board" — confirm the task disappears from "Main Board" but still exists on "Sprint 12".
8. Open the task on "Sprint 12", click "Delete permanently" in the edit modal, confirm the task is now gone from both boards.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/components/kanban/kanban-board.tsx frontend/src/components/kanban/kanban-column.tsx frontend/src/components/kanban/task-card.tsx frontend/src/components/kanban/task-form.tsx frontend/src/components/kanban/column-settings-modal.tsx frontend/src/components/kanban/add-to-board-modal.tsx
git commit -m "feat: board-aware kanban UI with add/remove-from-board and delete-permanently actions"
```

---

### Task 9: Playwright end-to-end verification

**Files:**
- Create: `/tmp/claude-1000/-home-tz-code-dev-bridge-manager/4fad2fe5-62e3-42e1-90ad-0dbab47aeee3/scratchpad/pw/multi_board_check.js`

**Interfaces:**
- Consumes: the fully-wired frontend from Task 8 running at `http://localhost:3000`, backend at `http://localhost:8080` (both via their dev containers).
- Produces: pass/fail console output plus screenshots under `.../scratchpad/pw/shots/`.

- [ ] **Step 1: Write the script**

```javascript
const { chromium } = require('playwright');

const BASE = 'http://localhost:3000';
const shotDir = '/tmp/claude-1000/-home-tz-code-dev-bridge-manager/4fad2fe5-62e3-42e1-90ad-0dbab47aeee3/scratchpad/pw/shots';
require('fs').mkdirSync(shotDir, { recursive: true });

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  const errors = [];
  page.on('console', msg => { if (msg.type() === 'error') errors.push(msg.text()); });
  page.on('pageerror', err => errors.push('pageerror: ' + err.message));
  page.on('response', async (res) => {
    if (res.url().includes('/api/v1/projects/') && res.status() >= 400) {
      errors.push(`HTTP ${res.status()} ${res.request().method()} ${res.url()}`);
    }
  });

  const shot = async (name) => page.screenshot({ path: `${shotDir}/${name}.png`, fullPage: true });

  console.log('1. login');
  await page.goto(`${BASE}/auth`);
  await page.fill('#email', 'john@example.com');
  await page.fill('#password', 'TestPass123!');
  await page.click('button[type="submit"]');
  await page.waitForURL(/dashboard/, { timeout: 15000 });

  console.log('2. open project 2 boards list');
  await page.goto(`${BASE}/dashboard/board/2`);
  await page.waitForTimeout(1500);
  await shot('mb01-boards-list');

  console.log('3. create a second board');
  await page.fill('input[placeholder="New board name..."]', 'Sprint 12');
  await page.getByRole('button', { name: 'New board' }).click();
  await page.waitForTimeout(1200);
  await shot('mb02-second-board-created');

  console.log('4. open Main Board');
  await page.getByText('Main Board', { exact: true }).click();
  await page.waitForURL(/\/dashboard\/board\/2\/\d+/, { timeout: 10000 });
  await page.waitForTimeout(1000);
  await shot('mb03-main-board-open');

  console.log('5. create a task on Main Board');
  await page.getByRole('button', { name: 'Add Task' }).first().click();
  await page.waitForTimeout(500);
  await page.fill('input[placeholder="Enter task title..."]', 'Cross-board task');
  await page.getByRole('button', { name: 'Create Task' }).click();
  await page.waitForTimeout(1000);
  await shot('mb04-task-created');

  console.log('6. add the task to Sprint 12 via the folder-plus action');
  const card = page.locator('div.group', { hasText: 'Cross-board task' }).first();
  await card.hover();
  await card.locator('button').filter({ has: page.locator('svg.lucide-folder-plus') }).click();
  await page.waitForTimeout(800);
  await shot('mb05-add-to-board-modal');
  await page.getByRole('button', { name: 'Add to board' }).click();
  await page.waitForTimeout(1000);
  await shot('mb06-added-to-sprint12');

  console.log('7. verify task also appears on Sprint 12');
  await page.goBack();
  await page.waitForTimeout(800);
  await page.getByText('Sprint 12', { exact: true }).click();
  await page.waitForURL(/\/dashboard\/board\/2\/\d+/, { timeout: 10000 });
  await page.waitForTimeout(1000);
  const onSprint12 = await page.getByText('Cross-board task').count();
  console.log('task visible on Sprint 12:', onSprint12 > 0);
  await shot('mb07-task-on-sprint12');

  console.log('8. remove the task from Sprint 12 only');
  const card2 = page.locator('div.group', { hasText: 'Cross-board task' }).first();
  await card2.hover();
  page.once('dialog', async (dialog) => { await dialog.accept(); });
  await card2.locator('button').filter({ has: page.locator('svg.lucide-trash2') }).click();
  await page.waitForTimeout(1000);
  await shot('mb08-removed-from-sprint12');

  console.log('9. verify task still exists on Main Board');
  await page.goBack();
  await page.waitForTimeout(800);
  await page.getByText('Main Board', { exact: true }).click();
  await page.waitForURL(/\/dashboard\/board\/2\/\d+/, { timeout: 10000 });
  await page.waitForTimeout(1000);
  const stillOnMain = await page.getByText('Cross-board task').count();
  console.log('task still visible on Main Board:', stillOnMain > 0);
  await shot('mb09-task-survives-on-main');

  console.log('10. delete the task permanently from Main Board');
  await page.getByText('Cross-board task').click();
  await page.waitForTimeout(500);
  page.once('dialog', async (dialog) => { await dialog.accept(); });
  await page.getByRole('button', { name: 'Delete permanently' }).click();
  await page.waitForTimeout(1000);
  await shot('mb10-deleted-permanently');

  console.log('11. verify task is gone from Main Board');
  const goneFromMain = await page.getByText('Cross-board task').count();
  console.log('task gone from Main Board:', goneFromMain === 0);

  console.log('CONSOLE/NETWORK ERRORS:', JSON.stringify(errors, null, 2));

  await browser.close();
  console.log('DONE');
})().catch(async (e) => {
  console.error('SCRIPT FAILED:', e.message);
  process.exit(1);
});
```

- [ ] **Step 2: Run it**

```bash
node /tmp/claude-1000/-home-tz-code-dev-bridge-manager/4fad2fe5-62e3-42e1-90ad-0dbab47aeee3/scratchpad/pw/multi_board_check.js
```

Expected: `DONE` printed, `task visible on Sprint 12: true`, `task still visible on Main Board: true`, `task gone from Main Board: true`, and an empty (or explainable) `CONSOLE/NETWORK ERRORS` array. Investigate and fix any non-empty errors array or `false` assertion before considering this plan complete — do not treat a Playwright script that reaches `DONE` with failed assertions as a pass.

- [ ] **Step 3: Review screenshots**

Check every `.png` in `.../scratchpad/pw/shots/` for visual correctness (boards list renders board names, the add-to-board modal shows both a board and column dropdown, the task genuinely disappears/reappears as expected at each step).

No commit for this task — the script lives in the scratchpad, not the repo, matching how `column_settings_check.js`/`column_settings_check2.js` were handled this session.
