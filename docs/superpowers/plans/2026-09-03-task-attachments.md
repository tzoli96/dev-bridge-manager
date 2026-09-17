# Plan: Task & Comment File Attachments

## Goal

Let users attach files (images, PDFs, Office docs, zip, plain text — max 10MB each, max 5 per upload) to a task or to an individual task comment, view/download them, and delete them (uploader or anyone with `tasks.update`). Fully implements the approved spec.

## Architecture

- New `attachments` table, FK'd to `tasks` (cascade) and optionally `task_comments` (cascade, nullable — null means task-level attachment).
- Files stored on disk under a new Docker named volume (`backend_uploads` → `/app/uploads/tasks/{taskId}/{uuid}{ext}`), not in the DB. DB row holds metadata + generated filename.
- Backend: new `AttachmentHandler` (upload/list/download/delete) mirroring the existing `TaskCommentHandler`/`TaskHandler` style; `kanban_dto_builder.go` extended so `TaskDTO.attachments` and `TaskCommentDTO.attachments` are populated for real (currently `Attachments: []interface{}{}` placeholder) using the same batch-load-then-group pattern already used for comments/time entries.
- Frontend: new `attachmentService` (service-per-resource pattern) + new `useAttachments` hook (hook-per-resource pattern, mirrors `useComments`) + new shared `<AttachmentList>` component used from both `TaskForm` (task-level attachments) and `CommentSection` (per-comment attachments). Upload/delete results are merged into the existing `Task`/`TaskComment` objects and pushed through the existing `updateTaskInStore`/`updateCommentInStore` setters — no new store actions.
- Auth for downloads: thumbnails/downloads go through `apiClient.getBlob()` (new method, `responseType: 'blob'`) so the existing `Authorization: Bearer` header (already attached to the shared axios instance by `setAuthToken`) covers them — no separate public/token-in-URL scheme needed.

## Tech Stack

Go/Fiber/GORM, PostgreSQL (golang-migrate), Next.js/React/TypeScript, Zustand+Immer, axios. `github.com/google/uuid` (already an indirect dependency in `go.sum`/`go.mod`, just not imported anywhere yet — used directly here for generating collision-free stored filenames; this is not a new dependency).

## Spec

`docs/superpowers/specs/2026-09-03-task-attachments-design.md` (approved).

## Global Constraints

- Never commit/push/create PRs/mutate external state — implementation only, per explicit ask.
- All Go/TS tooling runs only inside `devbridge_backend` / `devbridge_frontend` containers, never on host.
- Extend established codebase flows; the file-per-entity handler convention, the service/hook-per-resource frontend convention, and the existing DTO-batch-loading pattern are all reused as-is — no new abstractions.
- Never touch production/shared data.
- **Testing approach deviation from the `writing-plans` default (documented, not accidental):** this repo has zero test files anywhere (`find backend -name "*_test.go"` and `find frontend/src -iname "*.test.ts*"` both return 0 results) — there is no existing unit-test convention to extend, and per "extend established flows first, don't introduce new abstractions," this plan does **not** introduce a Go/TS test framework. Verification is: `docker exec devbridge_backend go build ./...` (and `go vet ./...`), `docker exec devbridge_frontend npx tsc --noEmit`, plus live Playwright verification — exactly the Testing Plan the approved spec already specifies. Every task below ends with the relevant one of these.

---

## Task 1: Migration — `attachments` table

**Files:**
- Create: `backend/migrations/000013_create_attachments_table.up.sql`
- Create: `backend/migrations/000013_create_attachments_table.down.sql`

**Steps:**

1. Write the up migration, following the exact style of `000012_create_boards_and_placements.up.sql` (SERIAL PK, `TIMESTAMP DEFAULT NOW()`, explicit `REFERENCES ... ON DELETE CASCADE`, explicit `CREATE INDEX`):

```sql
-- backend/migrations/000013_create_attachments_table.up.sql

CREATE TABLE attachments (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    comment_id INTEGER REFERENCES task_comments(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(127) NOT NULL,
    size BIGINT NOT NULL,
    uploaded_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_attachments_task_id ON attachments(task_id);
CREATE INDEX idx_attachments_comment_id ON attachments(comment_id);
```

2. Write the down migration:

```sql
-- backend/migrations/000013_create_attachments_table.down.sql

DROP TABLE IF EXISTS attachments;
```

3. Verify: `docker compose -f .docker/docker-compose.yml restart backend` and check backend logs for a clean migration run (`docker logs devbridge_backend --tail 30`) — no `❌ Migration failed` line. `RunMigrations` (`backend/internal/database/migrate.go`) picks this up automatically via `migrate.New("file://migrations", ...)`; no registration needed.

---

## Task 2: Backend model — `Attachment` + `AttachmentDTO`

**Files:**
- Create: `backend/internal/models/attachment.go`

**Interfaces:**
- Produces: `models.Attachment` (GORM model), `models.AttachmentDTO` (response DTO), consumed by Task 3–5.

**Steps:**

1. Create the file. `UploadedBy`/`UploadedByID` splits into an ID + a `*TaskUserRefDTO` exactly like the existing `TaskCommentDTO.UserID`/`TaskCommentDTO.User` and `TimeEntryDTO.UserID`/`TimeEntryDTO.User` pattern in `backend/internal/models/kanban.go` (the approved spec's `AttachmentDTO.UploadedBy string` was display-name-only, which can't be compared against `useAuth().user.id` on the frontend for the "uploader can delete" check — this refinement fixes that gap using the codebase's own established convention, no scope change):

```go
package models

import "time"

type Attachment struct {
	ID           uint `gorm:"primaryKey"`
	TaskID       uint `gorm:"not null;index"`
	CommentID    *uint `gorm:"index"`
	Filename     string
	OriginalName string
	MimeType     string
	Size         int64
	UploadedBy   uint
	CreatedAt    time.Time
}

func (Attachment) TableName() string { return "attachments" }

type AttachmentDTO struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"taskId"`
	CommentID    string          `json:"commentId,omitempty"`
	OriginalName string          `json:"originalName"`
	MimeType     string          `json:"mimeType"`
	Size         int64           `json:"size"`
	UploadedByID string          `json:"uploadedById"`
	UploadedBy   *TaskUserRefDTO `json:"uploadedBy,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	DownloadUrl  string          `json:"downloadUrl"`
}
```

2. Verify: `docker exec devbridge_backend go build ./...` — will fail at this point only if there's a syntax error in this file alone (nothing references it yet), confirm no errors.

---

## Task 3: Backend — extend `kanban_dto_builder.go` to load real attachments

**Files:**
- Modify: `backend/internal/handlers/kanban_dto_builder.go`

**Interfaces:**
- Consumes: `models.Attachment` (Task 2).
- Produces: `buildAttachmentDTO`, `taskProjectID` helpers; changes the signatures of `buildCommentDTO` and `buildTaskDTO`. Consumed by Task 4 (`task_comment_handler.go`) and Task 5 (`attachment_handler.go`); `task_handler.go`'s external calls to `loadSingleTaskDTO`/`loadTaskDTOs` are unaffected (those signatures stay the same).

**Steps:**

1. Add `buildAttachmentDTO`, right after `buildTimeEntryDTO`. `DownloadUrl` is built relative to the API base URL — matching how every other service call already passes endpoints to `apiClient` (`/projects/${id}/...`, no `/api/v1` prefix, since `apiClient`'s `baseURL` already includes that segment):

```go
func buildAttachmentDTO(a models.Attachment, users map[uint]models.User, projectID uint) models.AttachmentDTO {
	commentID := ""
	if a.CommentID != nil {
		commentID = models.IDToStr(*a.CommentID)
	}
	return models.AttachmentDTO{
		ID:           models.IDToStr(a.ID),
		TaskID:       models.IDToStr(a.TaskID),
		CommentID:    commentID,
		OriginalName: a.OriginalName,
		MimeType:     a.MimeType,
		Size:         a.Size,
		UploadedByID: models.IDToStr(a.UploadedBy),
		UploadedBy:   userRefDTO(a.UploadedBy, users),
		CreatedAt:    a.CreatedAt,
		DownloadUrl:  "/projects/" + models.IDToStr(projectID) + "/attachments/" + models.IDToStr(a.ID) + "/download",
	}
}

// taskProjectID looks up a task's project id (needed by call sites — comment
// handlers — that only have a task/comment id in scope, not the full Task row).
func taskProjectID(taskID uint) uint {
	var projectID uint
	database.GetDB().Model(&models.Task{}).Where("id = ?", taskID).Select("project_id").Scan(&projectID)
	return projectID
}
```

2. Change `buildCommentDTO`'s signature to accept the comment's attachments + the owning project id, and add `Attachments` to the returned DTO (this requires adding an `Attachments []AttachmentDTO` field to `TaskCommentDTO` in `kanban.go` — folded into this step since it's a one-line addition to the same struct already being touched conceptually; add it right after `IsEdited bool` in `TaskCommentDTO`, `json:"attachments"`):

```go
func buildCommentDTO(c models.TaskComment, users map[uint]models.User, projectID uint, attachments []models.Attachment) models.TaskCommentDTO {
	attachmentDTOs := make([]models.AttachmentDTO, 0, len(attachments))
	for _, a := range attachments {
		attachmentDTOs = append(attachmentDTOs, buildAttachmentDTO(a, users, projectID))
	}
	return models.TaskCommentDTO{
		ID:          models.IDToStr(c.ID),
		TaskID:      models.IDToStr(c.TaskID),
		Content:     c.Content,
		HTMLContent: c.HTMLContent,
		UserID:      models.IDToStr(c.UserID),
		User:        userRefDTO(c.UserID, users),
		IsEdited:    c.IsEdited,
		Attachments: attachmentDTOs,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
```

   In `backend/internal/models/kanban.go`, add to `TaskCommentDTO`:
   ```go
   Attachments []AttachmentDTO `json:"attachments"`
   ```
   (insert after the `IsEdited bool `json:"isEdited"`` line).

3. Change `buildTaskDTO`'s signature to take the task's own (comment-less) attachments plus a `commentID -> []Attachment` map (for nested comment attachments), and replace the `Attachments: []interface{}{}` placeholder:

```go
func buildTaskDTO(t models.Task, placement *models.TaskPlacement, comments []models.TaskComment, entries []models.TaskTimeEntry, taskAttachments []models.Attachment, commentAttachments map[uint][]models.Attachment, users map[uint]models.User) models.TaskDTO {
	commentDTOs := make([]models.TaskCommentDTO, 0, len(comments))
	for _, c := range comments {
		commentDTOs = append(commentDTOs, buildCommentDTO(c, users, t.ProjectID, commentAttachments[c.ID]))
	}

	entryDTOs := make([]models.TimeEntryDTO, 0, len(entries))
	var loggedHours float64
	for _, e := range entries {
		entryDTOs = append(entryDTOs, buildTimeEntryDTO(e, users))
		loggedHours += e.Hours
	}

	attachmentDTOs := make([]models.AttachmentDTO, 0, len(taskAttachments))
	for _, a := range taskAttachments {
		attachmentDTOs = append(attachmentDTOs, buildAttachmentDTO(a, users, t.ProjectID))
	}

	columnID := ""
	position := 0
	if placement != nil {
		columnID = models.IDToStr(placement.ColumnID)
		position = placement.Position
	}

	return models.TaskDTO{
		// ...unchanged fields...
		Comments:       commentDTOs,
		Attachments:    attachmentDTOs, // was: []interface{}{}
		Position:       position,
		// ...unchanged fields...
	}
}
```
   Also change `TaskDTO.Attachments` in `kanban.go` from `[]interface{} `json:"attachments"`` to `[]AttachmentDTO `json:"attachments"``.

4. Update all three loaders to fetch + group attachments alongside their existing comments/entries queries, and pass them into `buildTaskDTO`. `loadTaskDTOs`:

```go
func loadTaskDTOs(projectID uint) ([]models.TaskDTO, error) {
	var tasks []models.Task
	if err := database.GetDB().Where("project_id = ? AND is_archived = false", projectID).
		Order("id ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return []models.TaskDTO{}, nil
	}

	taskIDs := make([]uint, len(tasks))
	userIDs := make([]uint, 0, len(tasks)*2)
	for i, t := range tasks {
		taskIDs[i] = t.ID
		userIDs = append(userIDs, t.CreatedBy, t.UpdatedBy)
		if t.AssigneeID != nil {
			userIDs = append(userIDs, *t.AssigneeID)
		}
	}

	var comments []models.TaskComment
	database.GetDB().Where("task_id IN ?", taskIDs).Order("created_at ASC").Find(&comments)

	var entries []models.TaskTimeEntry
	database.GetDB().Where("task_id IN ?", taskIDs).Order("date DESC").Find(&entries)

	var attachments []models.Attachment
	database.GetDB().Where("task_id IN ?", taskIDs).Order("created_at ASC").Find(&attachments)

	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
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
	taskAttachmentsByTask := make(map[uint][]models.Attachment)
	commentAttachmentsByComment := make(map[uint][]models.Attachment)
	for _, a := range attachments {
		if a.CommentID == nil {
			taskAttachmentsByTask[a.TaskID] = append(taskAttachmentsByTask[a.TaskID], a)
		} else {
			commentAttachmentsByComment[*a.CommentID] = append(commentAttachmentsByComment[*a.CommentID], a)
		}
	}

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		dtos = append(dtos, buildTaskDTO(t, nil, commentsByTask[t.ID], entriesByTask[t.ID], taskAttachmentsByTask[t.ID], commentAttachmentsByComment, users))
	}
	return dtos, nil
}
```

   Apply the identical pattern (attachments query on the same ID set already being queried, split by `CommentID == nil`, `userIDs` extended with `a.UploadedBy`, extra params passed to `buildTaskDTO`) to `loadBoardTaskDTOs` (query key: `rowIDs`) and `loadSingleTaskDTO` (single-task query: `Where("task_id = ?", task.ID)`, no grouping needed — pass the filtered slices/maps directly).

5. Verify: `docker exec devbridge_backend go build ./...` — expect compile errors from `task_comment_handler.go`'s now-mismatched `buildCommentDTO` calls; that's expected and fixed in Task 4. Confirm the *only* errors are in `task_comment_handler.go` (proves `kanban_dto_builder.go`, `kanban.go`, and all `loadXDTOs`/`loadSingleTaskDTO` call sites in `task_handler.go`/`board_handler.go`/`kanban_handler.go` compile clean).

---

## Task 4: Backend — update comment handlers for the new `buildCommentDTO` signature

**Files:**
- Modify: `backend/internal/handlers/task_comment_handler.go`
- Modify: `backend/internal/models/kanban.go` (already touched in Task 3 step 2 for the `TaskCommentDTO.Attachments` field — no further model change here)

**Steps:**

1. `GetComments`: fetch attachments for the loaded comments, group by `comment_id`, resolve `projectID` once via `taskProjectID`:

```go
func (h *TaskCommentHandler) GetComments(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var comments []models.TaskComment
	if err := database.GetDB().Where("task_id = ?", taskID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading comments"})
	}

	commentIDs := make([]uint, len(comments))
	userIDs := make([]uint, 0, len(comments))
	for i, cm := range comments {
		commentIDs[i] = cm.ID
		userIDs = append(userIDs, cm.UserID)
	}

	var attachments []models.Attachment
	if len(commentIDs) > 0 {
		database.GetDB().Where("comment_id IN ?", commentIDs).Order("created_at ASC").Find(&attachments)
	}
	attachmentsByComment := make(map[uint][]models.Attachment)
	for _, a := range attachments {
		attachmentsByComment[*a.CommentID] = append(attachmentsByComment[*a.CommentID], a)
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)
	projectID := taskProjectID(uint(taskID))

	dtos := make([]models.TaskCommentDTO, 0, len(comments))
	for _, cm := range comments {
		dtos = append(dtos, buildCommentDTO(cm, users, projectID, attachmentsByComment[cm.ID]))
	}

	return c.JSON(dtos)
}
```

2. `CreateComment`: a brand-new comment has no attachments yet, pass `nil`:

```go
	users := loadUsersByIDs([]uint{comment.UserID})
	projectID := taskProjectID(uint(taskID))
	return c.Status(201).JSON(buildCommentDTO(comment, users, projectID, nil))
```
   (replaces the existing final two lines of `CreateComment`.)

3. `UpdateComment`: reload the comment's existing attachments (editing text shouldn't drop them from the response) and resolve `projectID` from `comment.TaskID`:

```go
	var attachments []models.Attachment
	database.GetDB().Where("comment_id = ?", comment.ID).Order("created_at ASC").Find(&attachments)

	userIDs := []uint{comment.UserID}
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)
	projectID := taskProjectID(comment.TaskID)
	return c.JSON(buildCommentDTO(comment, users, projectID, attachments))
```
   (replaces the existing final two lines of `UpdateComment`.)

4. `DeleteComment` is unchanged — deleting a `TaskComment` row cascades to its `attachments` rows via the migration's `ON DELETE CASCADE`, but **not** to the files on disk. Add a pre-delete step to remove the comment's attachment files before deleting the row, mirroring the cleanup style used in Task 5's `DeleteAttachment`:

```go
func (h *TaskCommentHandler) DeleteComment(c *fiber.Ctx) error {
	commentID, err := strconv.Atoi(c.Params("commentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid comment ID"})
	}

	var attachments []models.Attachment
	database.GetDB().Where("comment_id = ?", commentID).Find(&attachments)
	for _, a := range attachments {
		path := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(int(a.TaskID)), a.Filename)
		if err := os.Remove(path); err != nil {
			log.Printf("⚠️ Failed to remove attachment file %s: %v", path, err)
		}
	}

	if err := database.GetDB().Delete(&models.TaskComment{}, commentID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting comment"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Comment deleted successfully"})
}
```
   Add `"log"`, `"os"`, `"path/filepath"` to the file's imports. `uploadsBaseDir` is defined in Task 5's `attachment_handler.go` (same package `handlers`, so it's already in scope — no re-declaration).

5. Verify: `docker exec devbridge_backend go build ./...` — this will still fail until Task 5 adds `uploadsBaseDir`; if doing these tasks in strict order, do Task 5 first or expect this specific one error here and confirm it disappears after Task 5.

---

## Task 5: Backend — `AttachmentHandler` (upload / list / download / delete)

**Files:**
- Create: `backend/internal/handlers/attachment_handler.go`

**Interfaces:**
- Consumes: `models.Attachment`/`models.AttachmentDTO` (Task 2), `buildAttachmentDTO`/`taskProjectID`/`loadUsersByIDs`/`currentUserID` (Task 3, and `task_handler.go`'s existing `currentUserID`).
- Produces: `AttachmentHandler` with `GetAttachments`, `UploadAttachments`, `DownloadAttachment`, `DeleteAttachment` methods, `NewAttachmentHandler()` constructor, `uploadsBaseDir` const (also used by Task 4). Consumed by Task 6 (`kanban_routes.go`).

**Steps:**

1. Create the file:

```go
// handlers/attachment_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	maxAttachmentSize  = 10 * 1024 * 1024 // 10MB
	maxAttachmentCount = 5
	uploadsBaseDir     = "uploads"
)

var allowedAttachmentMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"application/pdf": true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/zip": true,
	"text/plain": true,
}

type AttachmentHandler struct{}

func NewAttachmentHandler() *AttachmentHandler {
	return &AttachmentHandler{}
}

// GetAttachments - GET /api/v1/projects/:id/tasks/:taskId/attachments
// Returns only task-level attachments (comment attachments come back nested
// in each comment's own DTO, via GetComments).
func (h *AttachmentHandler) GetAttachments(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var attachments []models.Attachment
	if err := database.GetDB().Where("task_id = ? AND comment_id IS NULL", taskID).Order("created_at ASC").Find(&attachments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading attachments"})
	}

	userIDs := make([]uint, 0, len(attachments))
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)
	projectID := taskProjectID(uint(taskID))

	dtos := make([]models.AttachmentDTO, 0, len(attachments))
	for _, a := range attachments {
		dtos = append(dtos, buildAttachmentDTO(a, users, projectID))
	}
	return c.JSON(dtos)
}

// UploadAttachments - POST /api/v1/projects/:id/tasks/:taskId/attachments
// multipart/form-data: field "files" (1-5 files), optional field "commentId".
func (h *AttachmentHandler) UploadAttachments(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	var commentID *uint
	if raw := c.FormValue("commentId"); raw != "" {
		id, err := models.StrToID(raw)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid comment ID"})
		}
		var comment models.TaskComment
		if err := database.GetDB().Where("id = ? AND task_id = ?", id, taskID).First(&comment).Error; err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Comment does not belong to this task"})
		}
		commentID = &id
	}

	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid multipart form"})
	}
	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "No files provided"})
	}
	if len(files) > maxAttachmentCount {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Maximum %d files per upload", maxAttachmentCount)})
	}
	for _, fh := range files {
		if fh.Size > maxAttachmentSize {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("%s exceeds the 10MB size limit", fh.Filename)})
		}
		if !allowedAttachmentMimeTypes[fh.Header.Get("Content-Type")] {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("%s has an unsupported file type", fh.Filename)})
		}
	}

	taskDir := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(taskID))
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error preparing upload directory"})
	}

	userID := currentUserID(c)
	saved := make([]models.Attachment, 0, len(files))
	for _, fh := range files {
		storedName := uuid.NewString() + strings.ToLower(filepath.Ext(fh.Filename))
		destPath := filepath.Join(taskDir, storedName)

		if err := saveMultipartFile(fh, destPath); err != nil {
			rollbackSavedAttachments(taskDir, saved)
			return c.Status(500).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Error saving %s", fh.Filename)})
		}

		attachment := models.Attachment{
			TaskID:       uint(taskID),
			CommentID:    commentID,
			Filename:     storedName,
			OriginalName: fh.Filename,
			MimeType:     fh.Header.Get("Content-Type"),
			Size:         fh.Size,
			UploadedBy:   userID,
		}
		if err := database.GetDB().Create(&attachment).Error; err != nil {
			os.Remove(destPath)
			rollbackSavedAttachments(taskDir, saved)
			return c.Status(500).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Error saving %s", fh.Filename)})
		}
		saved = append(saved, attachment)
	}

	users := loadUsersByIDs([]uint{userID})
	dtos := make([]models.AttachmentDTO, 0, len(saved))
	for _, a := range saved {
		dtos = append(dtos, buildAttachmentDTO(a, users, task.ProjectID))
	}
	return c.Status(201).JSON(dtos)
}

func rollbackSavedAttachments(taskDir string, saved []models.Attachment) {
	for _, a := range saved {
		os.Remove(filepath.Join(taskDir, a.Filename))
		database.GetDB().Delete(&a)
	}
}

func saveMultipartFile(fh *multipart.FileHeader, destPath string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// DownloadAttachment - GET /api/v1/projects/:id/attachments/:attachmentId/download
func (h *AttachmentHandler) DownloadAttachment(c *fiber.Ctx) error {
	attachmentID, err := strconv.Atoi(c.Params("attachmentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid attachment ID"})
	}

	var attachment models.Attachment
	if err := database.GetDB().First(&attachment, attachmentID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Attachment not found"})
	}

	path := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(int(attachment.TaskID)), attachment.Filename)
	c.Set("Content-Type", attachment.MimeType)
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, attachment.OriginalName))
	return c.SendFile(path)
}

// DeleteAttachment - DELETE /api/v1/projects/:id/attachments/:attachmentId
// Allowed for the uploader, or anyone with tasks.update (checked in-handler,
// not via route middleware, since it depends on who uploaded the specific row).
func (h *AttachmentHandler) DeleteAttachment(c *fiber.Ctx) error {
	attachmentID, err := strconv.Atoi(c.Params("attachmentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid attachment ID"})
	}

	var attachment models.Attachment
	if err := database.GetDB().First(&attachment, attachmentID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Attachment not found"})
	}

	userID := currentUserID(c)
	if userID != attachment.UploadedBy {
		allowed, err := services.NewPermissionService().CheckUserPermission(userID, "tasks.update")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to check permissions"})
		}
		if !allowed {
			return c.Status(403).JSON(fiber.Map{"success": false, "message": "You don't have permission to delete this attachment"})
		}
	}

	if err := database.GetDB().Delete(&attachment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting attachment"})
	}

	path := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(int(attachment.TaskID)), attachment.Filename)
	if err := os.Remove(path); err != nil {
		log.Printf("⚠️ Failed to remove attachment file %s: %v", path, err)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Attachment deleted successfully"})
}
```

2. Verify: `docker exec devbridge_backend go build ./...` — should now succeed with zero errors (Task 4's `uploadsBaseDir` reference now resolves). Also run `docker exec devbridge_backend go vet ./...` and `docker exec devbridge_backend go mod tidy` (promotes `google/uuid` from `// indirect` since it's now directly imported — a metadata-only `go.mod`/`go.sum` change, not a new dependency).

---

## Task 6: Backend — register attachment routes

**Files:**
- Modify: `backend/internal/routes/kanban_routes.go`

**Steps:**

1. Instantiate the handler and add the 4 routes, matching the existing style (`projects.Get/Post/Put/Delete`, `middleware.RequirePermission("tasks.update")` only on upload — list/download are open to anyone with a valid JWT on the project group, delete does its own in-handler check per Task 5):

```go
	attachmentHandler := handlers.NewAttachmentHandler()
```
   (add next to the other `handlers.NewXHandler()` lines)

```go
	// Attachments
	projects.Get("/:id/tasks/:taskId/attachments", attachmentHandler.GetAttachments)
	projects.Post("/:id/tasks/:taskId/attachments", middleware.RequirePermission("tasks.update"), attachmentHandler.UploadAttachments)
	projects.Get("/:id/attachments/:attachmentId/download", attachmentHandler.DownloadAttachment)
	projects.Delete("/:id/attachments/:attachmentId", attachmentHandler.DeleteAttachment)
```
   (add after the "Comments" block, before "Time entries")

2. Verify: `docker exec devbridge_backend go build ./...`.

---

## Task 7: Backend infra — upload volume + body limit

**Files:**
- Modify: `.docker/docker-compose.yml`
- Modify: `backend/cmd/server/main.go`

**Steps:**

1. In `.docker/docker-compose.yml`, add a named volume mount to the `backend` service (alongside the existing `../backend:/app` bind mount) and declare it at the top level next to `postgres_data`:

```yaml
  backend:
    ...
    volumes:
      - ../backend:/app
      - backend_uploads:/app/uploads
```
```yaml
volumes:
  postgres_data:
    driver: local
  backend_uploads:
    driver: local
```

2. In `backend/cmd/server/main.go`, raise Fiber's body limit (currently unset → ~4MB default, too small for 5×10MB uploads):

```go
	app := fiber.New(fiber.Config{
		AppName:      "Dev Bridge Manager v1.0",
		ErrorHandler: errorHandler,
		BodyLimit:    50 * 1024 * 1024, // 50MB — up to 5 attachments x 10MB each
	})
```

3. Verify: `docker compose -f .docker/docker-compose.yml up -d --build backend` (recreates the container with the new volume + config), then `docker exec devbridge_backend ls -la /app/uploads` should succeed (directory exists, even if empty — GORM/the handler's `os.MkdirAll` creates the `tasks/` subdir on first upload) and `docker exec devbridge_backend go build ./...` still passes.

---

## Task 8: Frontend types — replace `TaskAttachment`, add `attachments` to `TaskComment`

**Files:**
- Modify: `frontend/src/types/kanban/task.types.ts`

**Steps:**

1. Replace the existing placeholder `TaskAttachment` interface (currently `{ id, taskId, filename, originalName, mimeType, size, url, uploadedBy: string, createdAt }`) to match the real `AttachmentDTO` shape from Task 2/3, using the same inline-object convention already used by `TaskComment.user`/`TimeEntry.user` (no new named type):

```ts
export interface TaskAttachment {
    id: string;
    taskId: string;
    commentId?: string;
    originalName: string;
    mimeType: string;
    size: number;
    uploadedById: string;
    uploadedBy?: {
        id: string;
        name: string;
        avatar?: string;
    };
    createdAt: string;
    downloadUrl: string;
}
```

2. Add `attachments: TaskAttachment[];` to the `TaskComment` interface (after `isEdited: boolean;`):

```ts
export interface TaskComment {
    id: string;
    taskId: string;
    content: string;
    htmlContent?: string;
    userId: string;
    user?: {
        id: string;
        name: string;
        avatar?: string;
    };
    isEdited: boolean;
    attachments: TaskAttachment[];
    createdAt: string;
    updatedAt: string;
}
```

3. Verify: `docker exec devbridge_frontend npx tsc --noEmit` — expect no new errors from this file alone (nothing consumes the changed shape yet).

---

## Task 9: Frontend — `apiClient.uploadFiles` + `apiClient.getBlob`

**Files:**
- Modify: `frontend/src/lib/api.ts`

**Steps:**

1. Add a multi-file upload method (field name `files`, optional extra form fields for `commentId`) right after the existing single-file `upload<T>` method:

```ts
    // Multi-file upload
    async uploadFiles<T>(
        endpoint: string,
        files: File[],
        extraFields?: Record<string, string>,
        onProgress?: (progress: number) => void
    ): Promise<T> {
        const formData = new FormData()
        files.forEach((file) => formData.append('files', file))
        if (extraFields) {
            Object.entries(extraFields).forEach(([key, value]) => formData.append(key, value))
        }

        const response = await this.client.post<T>(endpoint, formData, {
            headers: {
                'Content-Type': 'multipart/form-data',
            },
            onUploadProgress: (progressEvent) => {
                if (onProgress && progressEvent.total) {
                    const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
                    onProgress(progress)
                }
            },
        })

        return response.data
    }

    // Authenticated blob fetch (thumbnails/downloads) — reuses the same
    // Authorization header the shared axios instance already carries.
    async getBlob(endpoint: string): Promise<Blob> {
        const response = await this.client.get(endpoint, { responseType: 'blob' })
        return response.data
    }
```

2. Verify: `docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 10: Frontend — `attachmentService`

**Files:**
- Create: `frontend/src/services/kanban/attachment.service.ts`
- Modify: `frontend/src/services/kanban/index.ts`

**Interfaces:**
- Consumes: `apiClient.uploadFiles`/`apiClient.getBlob` (Task 9), `TaskAttachment` (Task 8).
- Produces: `attachmentService`, consumed by Task 11 (`useAttachments`).

**Steps:**

1. Create the service, following the exact style of `comment.service.ts` (plain object of async methods, `projectId` first, template-literal URLs matching the routes registered in Task 6):

```ts
import { apiClient } from '@/lib/api';
import type { TaskAttachment } from '@/types/kanban';

export const attachmentService = {
    async getAttachments(projectId: string, taskId: string): Promise<TaskAttachment[]> {
        return apiClient.get(`/projects/${projectId}/tasks/${taskId}/attachments`);
    },

    async uploadAttachments(
        projectId: string,
        taskId: string,
        files: File[],
        commentId?: string,
        onProgress?: (progress: number) => void
    ): Promise<TaskAttachment[]> {
        return apiClient.uploadFiles(
            `/projects/${projectId}/tasks/${taskId}/attachments`,
            files,
            commentId ? { commentId } : undefined,
            onProgress
        );
    },

    // downloadUrl comes straight from the attachment's own `downloadUrl` field
    // (already the correct relative path — see buildAttachmentDTO), so no
    // URL-building is duplicated here.
    async downloadAttachment(downloadUrl: string): Promise<Blob> {
        return apiClient.getBlob(downloadUrl);
    },

    async deleteAttachment(projectId: string, attachmentId: string): Promise<void> {
        return apiClient.delete(`/projects/${projectId}/attachments/${attachmentId}`);
    },
};
```

2. Add the export to `frontend/src/services/kanban/index.ts`:

```ts
export { attachmentService } from './attachment.service';
```

3. Verify: `docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 11: Frontend — `useAttachments` hook

**Files:**
- Create: `frontend/src/hooks/kanban/use-attachments.ts`
- Modify: `frontend/src/hooks/kanban/index.ts`

**Interfaces:**
- Consumes: `attachmentService` (Task 10), `useKanbanStore`'s existing `tasks`/`comments`/`updateTaskInStore`/`updateCommentInStore` (no new store actions).
- Produces: `useAttachments(projectId)`, consumed by Task 12 (`TaskForm`) and Task 13 (`CommentSection`).

**Steps:**

1. Create the hook, mirroring `use-comments.ts`'s structure (local `isLoading`/`error`, store sync via existing setters). Upload/delete merge the changed `attachments` array into the current `Task` or `TaskComment` object (read live from the store) and push it back through the existing setter — this is what avoids adding new store actions:

```ts
'use client';

import { useState, useCallback } from 'react';
import { attachmentService } from '@/services/kanban';
import { useKanbanStore } from '@/stores/kanban';
import type { TaskAttachment } from '@/types/kanban';

interface UseAttachmentsReturn {
    isLoading: boolean;
    error: string | null;
    uploadAttachments: (taskId: string, files: File[], commentId?: string) => Promise<TaskAttachment[]>;
    deleteAttachment: (taskId: string, attachmentId: string, commentId?: string) => Promise<void>;
}

export const useAttachments = (projectId: string): UseAttachmentsReturn => {
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const { tasks, comments, updateTaskInStore, updateCommentInStore } = useKanbanStore();

    const uploadAttachments = useCallback(async (
        taskId: string,
        files: File[],
        commentId?: string
    ): Promise<TaskAttachment[]> => {
        setIsLoading(true);
        setError(null);
        try {
            const uploaded = await attachmentService.uploadAttachments(projectId, taskId, files, commentId);

            if (commentId) {
                const comment = comments.find(c => c.id === commentId);
                if (comment) {
                    updateCommentInStore({ ...comment, attachments: [...comment.attachments, ...uploaded] });
                }
            } else {
                const task = tasks.find(t => t.id === taskId);
                if (task) {
                    updateTaskInStore({ ...task, attachments: [...task.attachments, ...uploaded] });
                }
            }

            return uploaded;
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to upload attachments');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, tasks, comments, updateTaskInStore, updateCommentInStore]);

    const deleteAttachment = useCallback(async (
        taskId: string,
        attachmentId: string,
        commentId?: string
    ): Promise<void> => {
        setIsLoading(true);
        setError(null);
        try {
            await attachmentService.deleteAttachment(projectId, attachmentId);

            if (commentId) {
                const comment = comments.find(c => c.id === commentId);
                if (comment) {
                    updateCommentInStore({ ...comment, attachments: comment.attachments.filter(a => a.id !== attachmentId) });
                }
            } else {
                const task = tasks.find(t => t.id === taskId);
                if (task) {
                    updateTaskInStore({ ...task, attachments: task.attachments.filter(a => a.id !== attachmentId) });
                }
            }
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to delete attachment');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, tasks, comments, updateTaskInStore, updateCommentInStore]);

    return { isLoading, error, uploadAttachments, deleteAttachment };
};
```

2. Add to `frontend/src/hooks/kanban/index.ts`:

```ts
export { useAttachments } from './use-attachments';
```

3. Verify: `docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 12: Frontend — shared `<AttachmentList>` component

**Files:**
- Create: `frontend/src/components/kanban/attachment-list.tsx`

**Interfaces:**
- Consumes: `TaskAttachment` (Task 8), `attachmentService.downloadAttachment` (Task 10), `Button` (`@/components/ui/button`).
- Produces: `AttachmentList`, consumed by Task 13 (`TaskForm`) and Task 14 (`CommentSection`).

**Steps:**

1. Create the component: image attachments get a thumbnail (fetched as an authenticated blob via `attachmentService.downloadAttachment`, converted with `URL.createObjectURL`, revoked on unmount); non-image attachments get a generic file icon. Every row shows original name + formatted size, a download button, and (only when `canDelete`) a delete button. `canDelete` is `canManage || attachment.uploadedById === currentUserId`, computed per-row by the caller-supplied `canManage` and `currentUserId` props:

```tsx
'use client';

import React, { useEffect, useState } from 'react';
import { attachmentService } from '@/services/kanban';
import { Button } from '@/components/ui/button';
import { Download, FileText, Trash2 } from 'lucide-react';
import type { TaskAttachment } from '@/types/kanban';

interface AttachmentListProps {
    attachments: TaskAttachment[];
    currentUserId?: string;
    canManage: boolean;
    onDelete: (attachmentId: string) => void | Promise<void>;
}

const isImage = (mimeType: string) => mimeType.startsWith('image/');

const formatSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const AttachmentThumbnail: React.FC<{ attachment: TaskAttachment }> = ({ attachment }) => {
    const [blobUrl, setBlobUrl] = useState<string | null>(null);

    useEffect(() => {
        if (!isImage(attachment.mimeType)) return;
        let objectUrl: string | null = null;
        let cancelled = false;

        attachmentService.downloadAttachment(attachment.downloadUrl).then((blob) => {
            if (cancelled) return;
            objectUrl = URL.createObjectURL(blob);
            setBlobUrl(objectUrl);
        }).catch(() => {});

        return () => {
            cancelled = true;
            if (objectUrl) URL.revokeObjectURL(objectUrl);
        };
    }, [attachment.downloadUrl, attachment.mimeType]);

    if (isImage(attachment.mimeType)) {
        return blobUrl
            ? <img src={blobUrl} alt={attachment.originalName} className="w-10 h-10 rounded object-cover" />
            : <div className="w-10 h-10 rounded bg-gray-100 animate-pulse" />;
    }

    return (
        <div className="w-10 h-10 rounded bg-gray-100 flex items-center justify-center">
            <FileText className="w-5 h-5 text-gray-400" />
        </div>
    );
};

export const AttachmentList: React.FC<AttachmentListProps> = ({ attachments, currentUserId, canManage, onDelete }) => {
    const handleDownload = async (attachment: TaskAttachment) => {
        const blob = await attachmentService.downloadAttachment(attachment.downloadUrl);
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = attachment.originalName;
        link.click();
        URL.revokeObjectURL(url);
    };

    if (attachments.length === 0) return null;

    return (
        <div className="space-y-2">
            {attachments.map((attachment) => {
                const canDelete = canManage || attachment.uploadedById === currentUserId;
                return (
                    <div key={attachment.id} className="flex items-center gap-3 rounded-lg border border-gray-200 p-2">
                        <AttachmentThumbnail attachment={attachment} />
                        <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{attachment.originalName}</p>
                            <p className="text-xs text-gray-500">{formatSize(attachment.size)}</p>
                        </div>
                        <Button variant="ghost" size="sm" onClick={() => handleDownload(attachment)} icon={Download} />
                        {canDelete && (
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => onDelete(attachment.id)}
                                icon={Trash2}
                                className="text-red-600 hover:bg-red-50"
                            />
                        )}
                    </div>
                );
            })}
        </div>
    );
};
```

2. Verify: `docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 13: Frontend — `TaskForm` attachments section

**Files:**
- Modify: `frontend/src/components/kanban/task-form.tsx`

**Steps:**

1. Add `useParams` (adopting the same pattern already used by `comment-section.tsx`) to get a real `projectId` — today `TaskForm` calls `useTasks('')` with an empty string, only for local `getTask` lookups. Add imports: `useParams` from `next/navigation`, `useRef` from `react` (alongside existing `useState`/`useEffect`), `useAttachments` from `@/hooks/kanban`, `usePermissions` from `@/hooks/auth/use-permissions`, `useAuth` from `@/contexts/AuthContext`, `AttachmentList` from `./attachment-list`, `Paperclip` from `lucide-react`.

```tsx
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask } = useTasks(projectId);
    const { uploadAttachments, deleteAttachment } = useAttachments(projectId);
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const canManageAttachments = hasPermission('tasks:edit', projectId);
    const currentTask = taskId ? getTask(taskId) : undefined;
    const fileInputRef = useRef<HTMLInputElement>(null);
```

2. Add upload handlers (component body, alongside the other handlers):

```tsx
    const handleAttachmentFiles = async (fileList: FileList | null) => {
        if (!fileList || fileList.length === 0 || !taskId) return;
        try {
            await uploadAttachments(taskId, Array.from(fileList));
        } catch (error) {
            console.error('Error uploading attachments:', error);
        }
        if (fileInputRef.current) fileInputRef.current.value = '';
    };

    const handleAttachmentDrop = (e: React.DragEvent) => {
        e.preventDefault();
        handleAttachmentFiles(e.dataTransfer.files);
    };
```

3. Render the attachments section only when editing an existing task (`taskId` present) — insert after the "Tags" section, before "Actions":

```tsx
            {taskId && (
                <div className="space-y-2">
                    <label className="block text-sm font-medium text-gray-700">
                        Attachments
                    </label>
                    <AttachmentList
                        attachments={currentTask?.attachments ?? []}
                        currentUserId={user ? String(user.id) : undefined}
                        canManage={canManageAttachments}
                        onDelete={(attachmentId) => deleteAttachment(taskId, attachmentId)}
                    />
                    <div
                        onDragOver={(e) => e.preventDefault()}
                        onDrop={handleAttachmentDrop}
                        className="rounded-md border-2 border-dashed border-gray-300 p-4 text-center text-sm text-gray-500"
                    >
                        <input
                            ref={fileInputRef}
                            type="file"
                            multiple
                            className="hidden"
                            onChange={(e) => handleAttachmentFiles(e.target.files)}
                        />
                        <Button type="button" variant="outline" icon={Paperclip} onClick={() => fileInputRef.current?.click()}>
                            Add files
                        </Button>
                        <p className="mt-1">or drag and drop (max 5 files, 10MB each)</p>
                    </div>
                </div>
            )}
```

4. Verify: `docker exec devbridge_frontend npx tsc --noEmit`, then live check per the "Manual verification" task at the end.

---

## Task 14: Frontend — `CommentSection` attach flow

**Files:**
- Modify: `frontend/src/components/kanban/comment-section.tsx`

**Steps:**

1. Add imports: `useRef`, `useAttachments` from `@/hooks/kanban`, `usePermissions` from `@/hooks/auth/use-permissions`, `useAuth` from `@/contexts/AuthContext`, `AttachmentList` from `./attachment-list`, `Paperclip` from `lucide-react`. `projectId` is already derived via `useParams` here.

```tsx
    const { uploadAttachments, deleteAttachment } = useAttachments(projectId);
    const { user } = useAuth();
    const { hasPermission } = usePermissions();
    const canManageAttachments = hasPermission('tasks:edit', projectId);
    const [pendingFiles, setPendingFiles] = useState<File[]>([]);
    const fileInputRef = useRef<HTMLInputElement>(null);
```

2. Change `handleAddComment` to upload any pending files against the just-created comment, using the return value from `addComment` (Task 15's fix makes this possible):

```tsx
    const handleAddComment = async () => {
        if (!newComment.trim()) return;

        try {
            const comment = await addComment(taskId, {
                content: newComment.trim(),
                htmlContent: newCommentHtml || newComment.trim()
            });
            if (pendingFiles.length > 0) {
                await uploadAttachments(taskId, pendingFiles, comment.id);
                setPendingFiles([]);
            }
            setNewComment('');
            setNewCommentHtml('');
        } catch (error) {
            console.error('Error adding comment:', error);
        }
    };
```

3. In the comment rendering loop, render each comment's attachments after its content block:

```tsx
                            <div className="prose prose-sm max-w-none">
                                <div dangerouslySetInnerHTML={{
                                    __html: comment.htmlContent || comment.content
                                }} />
                            </div>

                            <AttachmentList
                                attachments={comment.attachments}
                                currentUserId={user ? String(user.id) : undefined}
                                canManage={canManageAttachments}
                                onDelete={(attachmentId) => deleteAttachment(taskId, attachmentId, comment.id)}
                            />
```

4. Add a file picker next to the composer, right before the "Add Comment Form" submit row:

```tsx
                    <div className="flex items-center gap-2">
                        <input
                            ref={fileInputRef}
                            type="file"
                            multiple
                            className="hidden"
                            onChange={(e) => setPendingFiles(Array.from(e.target.files ?? []))}
                        />
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            icon={Paperclip}
                            onClick={() => fileInputRef.current?.click()}
                        >
                            {pendingFiles.length > 0 ? `${pendingFiles.length} file(s) selected` : 'Attach files'}
                        </Button>
                    </div>
```

5. Verify: `docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 15: Frontend — `addComment` return type fix

**Files:**
- Modify: `frontend/src/hooks/kanban/use-comments.ts`

**Interfaces:**
- Consumes: nothing new.
- Produces: `addComment` now returns `Promise<TaskComment>` instead of `Promise<void>` — a backward-compatible change (existing callers ignoring the return value are unaffected); required by Task 14's upload-after-create flow.

**Steps:**

1. Change the `UseCommentsReturn` interface's `addComment` signature and the implementation:

```ts
    addComment: (taskId: string, data: CreateCommentData) => Promise<TaskComment>;
```

```ts
    const addComment = useCallback(async (taskId: string, data: CreateCommentData): Promise<TaskComment> => {
        setIsLoading(true);
        setError(null);

        try {
            const comment = await commentService.createComment(projectId, taskId, data);
            addCommentToStore(comment);
            return comment;
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to add comment');
            throw err;
        } finally {
            setIsLoading(false);
        }
    }, [projectId, addCommentToStore]);
```

2. Add `TaskComment` to the file's type-only import from `@/types/kanban` (currently only imports `CreateCommentData, UpdateCommentData`).

3. Verify: `docker exec devbridge_frontend npx tsc --noEmit`.

---

## Task 16: Full verification pass

**Steps:**

1. `docker exec devbridge_backend go build ./...` — zero errors.
2. `docker exec devbridge_backend go vet ./...` — zero errors.
3. `docker exec devbridge_frontend npx tsc --noEmit` — zero errors.
4. Live Playwright verification against the running stack (`docker compose -f .docker/docker-compose.yml up -d`), covering the approved spec's Testing Plan:
   - Upload an image to a task → thumbnail renders in `<AttachmentList>`.
   - Upload a non-image (e.g. a `.txt` or `.pdf`) to a task → generic file icon + name + size renders; download button fetches and saves it.
   - Upload a file to a comment (via the comment composer's attach button) → appears nested under that comment after submit.
   - Attempt to upload a 6th file in one batch, or a file over 10MB, or a disallowed mime type → each is rejected with a clear error and no partial DB rows/files are left behind (check `docker exec devbridge_backend ls uploads/tasks/<taskId>` and the `attachments` table row count match).
   - As the uploader (non-privileged role): delete own attachment → succeeds.
   - As a different non-privileged user: attempt to delete someone else's attachment → 403, delete button not even rendered (`canManage` false, `uploadedById` mismatch).
   - As a `tasks.update`-holding role (e.g. manager): delete someone else's attachment → succeeds.
   - Reload the page / refetch the task → attachments persist (round-trip through `GetTasks`/`GetTask`/`GetComments`, not just the optimistic store update).

## Self-Review Checklist

- [ ] Every task has exact file paths (Create/Modify) — no "somewhere in the frontend" style placeholders.
- [ ] Every new/changed function signature is spelled out in full (Go and TS).
- [ ] `AttachmentDTO`/`TaskAttachment` shapes match 1:1 between backend JSON tags and frontend interface fields.
- [ ] `DownloadUrl`/`downloadUrl` format is consistent everywhere it's produced (`buildAttachmentDTO`) and consumed (`attachmentService.downloadAttachment`, `<AttachmentThumbnail>`) — relative path, no `/api/v1` prefix, matches how every other service call already talks to `apiClient`.
- [ ] No new store actions were introduced — attachment state sync reuses `updateTaskInStore`/`updateCommentInStore` exactly as `Task`/`TaskComment` updates already do elsewhere.
- [ ] No new test framework introduced; verification steps match the repo's actual, established convention (`go build`, `go vet`, `tsc --noEmit`, live Playwright) — explicitly called out as a deviation from the skill's TDD default, with the reason (zero existing test files) stated in Global Constraints.
- [ ] `google/uuid` usage is flagged as "already indirect, not a new dependency," with a `go mod tidy` step to make that explicit in `go.mod`.
- [ ] File-deletion-on-DB-deletion is handled in both directions that can orphan a file: attachment delete (Task 5) and comment delete cascading to its attachments (Task 4 step 4) — task delete/board-placement-removal cascades were already confirmed out of scope for this plan since `ON DELETE CASCADE` on `attachments.task_id` cleans up the DB rows, and stray files under `uploads/tasks/{taskId}/` for a fully-deleted task are an accepted (spec-approved) minor disk-cleanup gap, not a correctness bug.
