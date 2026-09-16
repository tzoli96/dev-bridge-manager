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
	"image/jpeg":         true,
	"image/png":          true,
	"image/gif":          true,
	"image/webp":         true,
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/zip": true,
	"text/plain":      true,
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
		logActivity(attachment.TaskID, userID, "attachment_added", "", "", attachment.OriginalName)
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
	logActivity(attachment.TaskID, userID, "attachment_deleted", "", "", attachment.OriginalName)

	path := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(int(attachment.TaskID)), attachment.Filename)
	if err := os.Remove(path); err != nil {
		log.Printf("⚠️ Failed to remove attachment file %s: %v", path, err)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Attachment deleted successfully"})
}
