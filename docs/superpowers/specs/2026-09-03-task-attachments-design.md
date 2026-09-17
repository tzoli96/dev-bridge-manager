# Task & Comment Attachments — Design

**Date:** 2026-09-03
**Status:** Approved for spec review

## Purpose

Tasks and comments currently have no working attachment support — `TaskDTO.Attachments` is a hardcoded empty `[]interface{}{}` and no upload flow exists anywhere in the app. This adds the ability to attach images and documents to a task, and separately to an individual comment on a task.

## Scope

In scope: uploading, listing, downloading, and deleting attachments on tasks and on comments; image preview for image attachments; generic file display for other attachment types.

Out of scope: attachment versioning, virus scanning, image resizing/thumbnail generation on the backend (the frontend renders the original file at display size), editing an attachment after upload (delete + re-upload only).

## Data Model

New GORM model, `backend/internal/models/attachment.go`:

```go
type Attachment struct {
    ID           uint      `gorm:"primaryKey"`
    TaskID       uint      `gorm:"index;not null"`
    CommentID    *uint     `gorm:"index"` // nil = task-level attachment; set = belongs to that comment
    Filename     string    `gorm:"not null"` // uuid-based name on disk, e.g. "3f2a...c1.png"
    OriginalName string    `gorm:"not null"` // filename as uploaded, shown in the UI
    MimeType     string    `gorm:"not null"`
    Size         int64     `gorm:"not null"` // bytes
    UploadedBy   uint      `gorm:"not null"` // FK users.id
    CreatedAt    time.Time
}
```

A single table covers both task-level and comment-level attachments: `CommentID` is nil for an attachment added directly to a task, and set for one added to a specific comment. This avoids duplicating the model/handler/routes for two attachment kinds.

Foreign keys: `TaskID` references `tasks.id`, `CommentID` references `task_comments.id`, both `ON DELETE CASCADE` — deleting a task or comment removes its attachment rows. The uploaded files themselves are cleaned up by the handler (see Deletion below), not by a DB-level trigger.

## Storage

Files are stored on local disk inside the backend container, under a dedicated Docker named volume — not the bind-mounted source tree — so uploaded files never land in the repo or get wiped by a container rebuild.

`.docker/docker-compose.yml`: add a `backend_uploads` named volume, mounted at `/app/uploads` in the `devbridge_backend` service (alongside the existing `../backend:/app` bind mount).

On-disk layout: `/app/uploads/tasks/{taskId}/{uuid}.{ext}`. The `{uuid}.{ext}` filename avoids collisions and avoids trusting user-supplied filenames on disk; `OriginalName` in the DB carries the human-readable name shown in the UI.

## Backend API

Four endpoints added to `backend/internal/routes/kanban_routes.go`, following the existing project-scoped, permission-gated pattern (`middleware.RequirePermission(...)` per route, same style as the existing task/comment routes):

```
POST   /api/v1/projects/:id/tasks/:taskId/attachments        (tasks.update)
GET    /api/v1/projects/:id/tasks/:taskId/attachments         (no extra permission — same visibility as GetTask)
GET    /api/v1/projects/:id/attachments/:attachmentId/download (no extra permission — same visibility as GetTask)
DELETE /api/v1/projects/:id/attachments/:attachmentId         (uploader OR tasks.update — see Deletion)
```

**Upload** (`POST .../attachments`): `multipart/form-data`. Accepts 1–5 files per request under field name `files`, plus an optional `commentId` form field (when present, the resulting rows get that `CommentID`; the comment is validated to belong to the same task). Server-side validation, in order:
- reject if more than 5 files in one request
- reject any file over 10MB
- reject any file whose MIME type isn't in the whitelist: `image/jpeg`, `image/png`, `image/gif`, `image/webp`, `application/pdf`, `application/msword`, `application/vnd.openxmlformats-officedocument.wordprocessingml.document`, `application/vnd.ms-excel`, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `application/zip`, `text/plain`

A validation failure rejects the whole request with 400 and a message naming the offending file — no partial upload. On success, each file is written to disk, one `Attachment` row is inserted per file, and the endpoint returns the created `AttachmentDTO[]`.

**List**: returns `AttachmentDTO[]` for a task (used to hydrate `TaskDTO.attachments`; `kanban_dto_builder.go`'s current `Attachments: []interface{}{}` is replaced with a real list built the same way tags/comments already are). Comment attachments are embedded in `TaskCommentDTO` the same way, not fetched separately.

**Download**: streams the file from disk with `Content-Type` set from the stored `MimeType` and `Content-Disposition: inline` (browser decides whether to render or download based on type; the frontend attachment list also offers an explicit "download" affordance using the same URL). Sits behind the existing `JWTMiddleware()` on the `projects` group — same auth as every other endpoint here.

**Deletion**: handler loads the `Attachment`, allows the request if `currentUserID(c) == attachment.UploadedBy` OR `services.NewPermissionService().CheckUserPermission(userID, "tasks.update")` returns true — the same permission-service call `middleware.RequirePermission` wraps, invoked directly in-handler since this rule (uploader-or-permission) doesn't map to a single static route-level permission. Deletes the DB row, then attempts to remove the file from disk; if the disk removal fails, the error is logged and the request still succeeds — an orphaned file on disk is a non-blocking cleanup issue, not a user-facing failure. This mirrors how other best-effort cleanup is already handled elsewhere in the codebase.

`AttachmentDTO`:
```go
type AttachmentDTO struct {
    ID           string `json:"id"`
    OriginalName string `json:"originalName"`
    MimeType     string `json:"mimeType"`
    Size         int64  `json:"size"`
    UploadedBy   string `json:"uploadedBy"` // display name, resolved like comment authors already are
    CreatedAt    string `json:"createdAt"`
    DownloadUrl  string `json:"downloadUrl"` // relative API path, e.g. /api/v1/projects/:id/attachments/:attachmentId/download
}
```

## Frontend

`frontend/src/types/kanban/task.types.ts`: replace `TaskAttachment` (already declared, currently unused) with fields matching `AttachmentDTO` above (`downloadUrl` instead of the current placeholder `url`/`filename`/`taskId` shape), and add it to `TaskComment`.

New shared component `frontend/src/components/kanban/attachment-list.tsx`:
- Props: `attachments: TaskAttachment[]`, `canDelete: (a: TaskAttachment) => boolean`, `onDelete: (id: string) => void`.
- Image MIME types render as a thumbnail; the thumbnail `src` is set via `URL.createObjectURL` on a blob fetched through the existing Authorization-header-based API client (`frontend/src/lib/api.ts`) — no new auth path, consistent with every other authenticated request in the app. Object URLs are revoked on unmount.
- Non-image types render as a file-type icon + `originalName` + human-readable size, with a download affordance that triggers the same authenticated blob fetch and saves it client-side.
- Each row shows a delete icon only when `canDelete(attachment)` is true.

Two integration points:
- **Task detail/edit form** (`task-form.tsx`): a "Csatolmányok" section below the description, with a drag-and-drop zone plus a file-picker button, rendering `<AttachmentList>` beneath it. Enabled only when editing an existing task (`taskId` present) — on the create-task form this section is hidden, since attachments need a task to attach to. `canDelete` = uploader-is-current-user OR `permissions.canEditTasks`.
- **Comment section** (`comment-section.tsx`): a small attach button next to the comment composer; selected files upload immediately after the comment is created (same endpoint, with `commentId` set), and each comment renders its own `<AttachmentList>` beneath its text.

Upload UX: a progress/disabled state on the drop zone while the request is in flight; validation errors (oversized file, disallowed type, more than 5 files) surface inline in the same style as existing form validation errors.

## Error Handling

- Backend rejects invalid uploads before writing anything to disk (whole-request validation pass first).
- Frontend performs the same size/type/count checks client-side before submitting, purely to avoid a wasted round-trip — the backend re-validates independently and is the actual gate.
- A failed disk write on upload rolls back: no DB row is created for a file that didn't save successfully; the endpoint returns a partial-failure response listing which files succeeded/failed so the UI can retry just the failed ones.

## Testing Plan

- Backend: `docker exec devbridge_backend go build ./...`; if a handler test pattern already exists for kanban handlers, add attachment handler tests following it (upload validation limits, delete permission logic).
- Frontend: `docker exec devbridge_frontend npx tsc --noEmit` (confirm no new errors beyond the existing baseline).
- Live verification via Playwright: upload an image to a task and confirm thumbnail rendering; upload a non-image file and confirm icon+name rendering and successful download; upload an image to a comment; attempt deletion as a non-uploader without edit permission (expect it hidden/denied) and as the uploader (expect it to succeed).
